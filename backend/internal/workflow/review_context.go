package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/pmezard/go-difflib/difflib"
)

// Tool capability checks retain user instructions independently of transcript
// compaction. Independent artifact review uses demand instead.
type ReviewInstruction struct {
	InputID       string                      `json:"input_id,omitempty"`
	Text          string                      `json:"text"`
	Attachments   []model.AttachmentReference `json:"attachments,omitempty"`
	DOMSelections []model.DOMSelection        `json:"dom_selections,omitempty"`
}

type reviewSourceFile struct {
	NodeKind string `json:"node_kind"`
	Mode     uint32 `json:"mode"`
	Hash     string `json:"hash"`
	Size     int    `json:"size"`
	Binary   bool   `json:"binary"`
	Content  string `json:"content,omitempty"`
}

type ReviewFileChange struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Diff string `json:"diff"`
}

// Snapshot proofs stay local; only demand, changes and ordered source content
// are projected into the independent model request.
type reviewPageProof struct {
	SlideID        string
	SourceHash     string
	DependencyHash string
}

type ReviewMaterial struct {
	Demand      string
	Changes     []ReviewFileChange
	Sources     []llm.ContentPart
	sourceHash  string
	sourceCount int
	pages       []reviewPageProof
	imageData   map[string]llm.ImageData
}

func (m ReviewMaterial) content() ([]llm.ContentPart, error) {
	changes := m.Changes
	if changes == nil {
		changes = []ReviewFileChange{}
	}
	raw, err := json.Marshal(changes)
	if err != nil {
		return nil, err
	}
	parts := []llm.ContentPart{
		{Type: "text", Text: "demand\n" + m.Demand},
		{Type: "text", Text: "changes\n" + string(raw)},
		{Type: "text", Text: "current_sources"},
	}
	return append(parts, m.Sources...), nil
}

func reviewBaselinePath(root, runID string) string {
	return filepath.Join(root, ".runtime", "review-baselines", hashBytes([]byte(runID))+".json")
}

// Match the project history boundary: runtime output, Git internals and saved
// versions are not authored artifacts. Other project files, including assets,
// are included, even when edited through run_command rather than PPT tools.
func reviewSourceFiles(ctx context.Context, root string) (map[string]reviewSourceFile, error) {
	release := pptmutation.ReadLockProject(root)
	defer release()
	files := map[string]reviewSourceFile{}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("review source must be a project directory")
	}
	total := 0
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		switch strings.Split(rel, "/")[0] {
		case ".runtime", ".git", ".run", ".commit-tmp", "versions":
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			return reviewMaterialError(fmt.Errorf("unsupported review source %s", rel))
		}
		// Refuse an incomplete snapshot rather than silently omitting artifacts.
		if info.Size() > 64<<20 || int64(total)+info.Size() > 128<<20 {
			return reviewMaterialError(errors.New("review source material exceeds 128 MiB snapshot budget"))
		}
		node, err := commandexec.ReadNode(path)
		if err != nil {
			return err
		}
		if node == nil {
			return fmt.Errorf("review source disappeared: %s", rel)
		}
		raw := node.Content
		total += len(raw)
		file := reviewSourceFile{NodeKind: node.Kind, Mode: node.Mode, Hash: hashBytes(raw), Size: len(raw), Binary: !utf8.Valid(raw) || strings.IndexByte(string(raw), 0) >= 0}
		if !file.Binary {
			file.Content = string(raw)
		}
		files[rel] = file
		return nil
	})
	return files, err
}

func ensureReviewBaseline(ctx context.Context, root, runID string, resuming bool) error {
	path := reviewBaselinePath(root, runID)
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if resuming {
		return errors.New("Run-start review baseline is missing; cumulative changes cannot be reconstructed")
	}
	files, err := reviewSourceFiles(ctx, root)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(files)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".baseline-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func reviewChanges(before, after map[string]reviewSourceFile) ([]ReviewFileChange, error) {
	paths := map[string]bool{}
	for path := range before {
		paths[path] = true
	}
	for path := range after {
		paths[path] = true
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	changes := []ReviewFileChange{}
	for _, path := range ordered {
		old, existed := before[path]
		next, exists := after[path]
		if existed && exists && old.Hash == next.Hash && old.Mode == next.Mode && old.NodeKind == next.NodeKind {
			continue
		}
		change := ReviewFileChange{Path: path, Kind: "modified"}
		if !existed {
			change.Kind = "added"
		}
		if !exists {
			change.Kind = "deleted"
		}
		if !old.Binary && !next.Binary {
			diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{A: difflib.SplitLines(old.Content), B: difflib.SplitLines(next.Content), FromFile: "run-start/" + path, ToFile: "current/" + path, Context: 3})
			if err != nil {
				return nil, err
			}
			change.Diff = diff
		} else {
			change.Diff = "Binary content changed; no text diff available."
		}
		if old.Mode != next.Mode || old.NodeKind != next.NodeKind {
			change.Diff += fmt.Sprintf("\nFilesystem: %s %04o -> %s %04o", old.NodeKind, old.Mode, next.NodeKind, next.Mode)
		}
		changes = append(changes, change)
	}
	return changes, nil
}

type reviewEvidenceRenderer func(context.Context, string, model.RunScope) (ToolResult, error)

func prepareReviewMaterial(ctx context.Context, state *RunState, demand string, render reviewEvidenceRenderer) (ReviewMaterial, error) {
	scope := state.scope
	if scope.Source.Kind == "" {
		scope = state.pack.Command.Scope
	}
	material := ReviewMaterial{Demand: demand, imageData: map[string]llm.ImageData{}}
	if state.reviewBaselineError != "" {
		return material, errors.New(state.reviewBaselineError)
	}
	raw, err := os.ReadFile(reviewBaselinePath(state.projectDir, state.runID))
	if err != nil {
		return material, err
	}
	var baseline map[string]reviewSourceFile
	if err := json.Unmarshal(raw, &baseline); err != nil {
		return material, err
	}
	current, err := reviewSourceFiles(ctx, state.projectDir)
	if err != nil {
		return material, err
	}
	material.sourceHash = hashCheckpointValue(current)
	material.sourceCount = len(current)
	material.Changes, err = reviewChanges(baseline, current)
	if err != nil {
		return material, err
	}
	addText := func(text string) {
		material.Sources = append(material.Sources, llm.ContentPart{Type: "text", Text: text})
	}
	addSource := func(label, path string) error {
		file, exists := current[path]
		if !exists {
			addText(label + " (" + path + ")\n[missing]")
			return nil
		}
		if file.Binary {
			return reviewMaterialError(fmt.Errorf("required review source %s is not text", path))
		}
		addText(label + " (" + path + ")\n" + file.Content)
		return nil
	}
	for _, resource := range []struct{ label, path string }{{"manifest", ".manifest.json"}, {"design", ".design.json"}, {"outline", ".outline.json"}} {
		if err := addSource(resource.label, resource.path); err != nil {
			return material, err
		}
	}
	outline, err := currentOutline(state.pack, state.tx)
	if err != nil {
		if errors.Is(err, spec.ErrInvalid) {
			return material, reviewMaterialError(err)
		}
		var syntax *json.SyntaxError
		if errors.As(err, &syntax) {
			return material, reviewMaterialError(err)
		}
		return material, fmt.Errorf("read review page directory: %w", err)
	}
	// Reading/deriving evidence may cover dependencies outside the editing scope.
	// This local scope authorizes only the backend renderer, never artifact edits.
	renderScope := scope
	renderScope.SlideIDs = []string{}
	locations := spec.FlattenOutline(outline)
	for _, loc := range locations {
		renderScope.SlideIDs = append(renderScope.SlideIDs, loc.Slide.ID)
	}
	imageBytes, renders := 0, 0
	for i, location := range locations {
		if err := ctx.Err(); err != nil {
			return material, err
		}
		id := location.Slide.ID
		html, exists := current[model.SlideHTMLPath(id)]
		addText(fmt.Sprintf("page %d (slide_id: %s)", i+1, id))
		if err := addSource("spec", model.SlideSpecPath(id)); err != nil {
			return material, err
		}
		if err := addSource("html", model.SlideHTMLPath(id)); err != nil {
			return material, err
		}
		if !exists {
			addText("screenshot\n[missing: HTML source is missing]")
			continue
		}
		if _, exists := current[model.SlideSpecPath(id)]; !exists {
			addText("screenshot\n[missing: Spec required for rendering is missing]")
			continue
		}
		proof, err := currentRenderProof(state.pack, state.projectDir, state.tx, id, html.Hash)
		if err != nil {
			return material, fmt.Errorf("read required render sources for %s: %w", id, err)
		}
		dependency := proof.DependencyHash()
		entry, loadErr := renderimage.Latest(state.projectDir, state.pack.Project.ID, id)
		if loadErr != nil && !errors.Is(loadErr, renderimage.ErrUnavailable) && !errors.Is(loadErr, os.ErrNotExist) {
			return material, loadErr
		}
		var pixels []byte
		if loadErr == nil && entry.SourceHash == html.Hash && entry.DependencyHash == dependency {
			_, pixels, loadErr = renderimage.Read(ctx, state.projectDir, state.pack.Project.ID, entry.ImageRef())
		} else {
			loadErr = renderimage.ErrUnavailable
		}
		if loadErr != nil {
			if !errors.Is(loadErr, renderimage.ErrUnavailable) && !errors.Is(loadErr, os.ErrNotExist) {
				return material, loadErr
			}
			if render == nil {
				return material, reviewMaterialError(fmt.Errorf("required review screenshot is missing or stale for %s", id))
			}
			if renders >= reviewMaxEvidenceRenders {
				return material, reviewMaterialError(errors.New("review evidence render budget exhausted"))
			}
			renders++
			result, err := render(ctx, id, renderScope)
			if err != nil {
				return material, err
			}
			// A completed render can report an observed artifact defect with pixels.
			// Infrastructure/reading/render execution failures never become verdicts.
			hasDiagnostic := false
			for _, e := range result.Evidence {
				if e.Kind == "render_diagnostic" {
					hasDiagnostic = true
				}
			}
			if !result.OK && !hasDiagnostic {
				failure := toolResultError(result)
				if model.IsCommonErrorCode(failure.Code) {
					return material, failure
				}
				return material, reviewMaterialError(failure)
			}
			entry, loadErr = renderimage.Latest(state.projectDir, state.pack.Project.ID, id)
			proof, err = currentRenderProof(state.pack, state.projectDir, state.tx, id, html.Hash)
			if err != nil {
				return material, fmt.Errorf("read rendered review sources for %s: %w", id, err)
			}
			dependency = proof.DependencyHash()
			if loadErr != nil || entry.SourceHash != html.Hash || entry.DependencyHash != dependency {
				if loadErr != nil && !errors.Is(loadErr, renderimage.ErrUnavailable) && !errors.Is(loadErr, os.ErrNotExist) {
					return material, loadErr
				}
				return material, reviewMaterialError(fmt.Errorf("review screenshot for %s does not match the source snapshot", id))
			}
			_, pixels, err = renderimage.Read(ctx, state.projectDir, state.pack.Project.ID, entry.ImageRef())
			if err != nil {
				if errors.Is(err, renderimage.ErrUnavailable) || errors.Is(err, os.ErrNotExist) {
					return material, reviewMaterialError(fmt.Errorf("read required review screenshot for %s: %w", id, err))
				}
				return material, fmt.Errorf("read required review screenshot for %s: %w", id, err)
			}
		}
		imageBytes += len(pixels)
		if imageBytes > 128<<20 {
			return material, reviewMaterialError(errors.New("review images exceed 128 MiB evidence budget; no image was omitted"))
		}
		material.imageData[entry.ImageRef()] = llm.ImageData{Bytes: pixels, MIMEType: "image/png"}
		addText(fmt.Sprintf("screenshot for page %d (slide_id: %s)", i+1, id))
		material.Sources = append(material.Sources, llm.ContentPart{Type: "image", ImageRef: entry.ImageRef(), MIMEType: "image/png", Detail: "high"})
		material.pages = append(material.pages, reviewPageProof{SlideID: id, SourceHash: html.Hash, DependencyHash: dependency})
	}
	if err := validateReviewEvidence(ctx, state, material); err != nil {
		return material, err
	}
	return material, nil
}

type reviewImageResolver map[string]llm.ImageData

func (r reviewImageResolver) ResolveImage(ctx context.Context, ref string) (llm.ImageData, error) {
	if err := ctx.Err(); err != nil {
		return llm.ImageData{}, err
	}
	data, ok := r[ref]
	if !ok {
		return llm.ImageData{}, llm.ErrImageReference
	}
	return llm.ImageData{Bytes: append([]byte(nil), data.Bytes...), MIMEType: data.MIMEType}, nil
}

func validateReviewEvidence(ctx context.Context, state *RunState, material ReviewMaterial) error {
	current, err := reviewSourceFiles(ctx, state.projectDir)
	if err != nil {
		return err
	}
	if hashCheckpointValue(current) != material.sourceHash {
		return reviewMaterialError(errors.New("artifacts changed during review; the assessment no longer applies to the current content"))
	}
	for _, page := range material.pages {
		proof, err := currentRenderProof(state.pack, state.projectDir, state.tx, page.SlideID, page.SourceHash)
		if err != nil {
			return err
		}
		if proof.DependencyHash() != page.DependencyHash {
			return reviewMaterialError(errors.New("render dependencies changed during review; the assessment is stale"))
		}
	}
	return ctx.Err()
}
