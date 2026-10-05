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

	"github.com/dasi0227/PPT-Agent/backend/internal/attachment"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/pmezard/go-difflib/difflib"
)

// User instructions are kept separately from the main agent's transcript so
// compaction cannot erase corrections that the reviewer needs to judge artifacts.
type ReviewInstruction struct {
	RunID         string                      `json:"run_id,omitempty"`
	Text          string                      `json:"text"`
	Attachments   []model.AttachmentReference `json:"attachments,omitempty"`
	DOMSelections []model.DOMSelection        `json:"dom_selections,omitempty"`
}

type reviewSourceFile struct {
	Hash    string `json:"hash"`
	Size    int    `json:"size"`
	Binary  bool   `json:"binary"`
	Content string `json:"content,omitempty"`
}

type ReviewFileChange struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	BeforeHash string `json:"before_hash,omitempty"`
	AfterHash  string `json:"after_hash,omitempty"`
	Binary     bool   `json:"binary,omitempty"`
	Diff       string `json:"diff,omitempty"`
}

type ReviewPage struct {
	SlideID              string                `json:"slide_id"`
	Number               int                   `json:"number"`
	HTMLExists           bool                  `json:"html_exists"`
	Render               *RenderedImageContext `json:"latest_render,omitempty"`
	RenderDependencyHash string                `json:"render_dependency_hash,omitempty"`
	Diagnostics          map[string]any        `json:"render_diagnostics,omitempty"`
}

type ReviewSource struct {
	Path string `json:"path"`
	reviewSourceFile
}

type ReviewImageEvidence struct {
	ImageRef     string `json:"image_ref"`
	SlideID      string `json:"slide_id,omitempty"`
	AttachmentID string `json:"attachment_id,omitempty"`
	Hash         string `json:"hash"`
}

type ReviewMaterial struct {
	SourceHash       string                `json:"-"`
	EvidenceVersion  string                `json:"evidence_version"`
	Scope            model.RunScope        `json:"task_scope"`
	Options          model.RunOptions      `json:"user_options"`
	UserInstructions []ReviewInstruction   `json:"user_instructions"`
	Demand           string                `json:"demand"`
	Changes          []ReviewFileChange    `json:"changes"`
	TaskChanges      []ReviewFileChange    `json:"task_changes"`
	Sources          []ReviewSource        `json:"current_sources"`
	Resources        map[string]bool       `json:"resource_availability"`
	Pages            []ReviewPage          `json:"pages"`
	ImageEvidence    []ReviewImageEvidence `json:"image_evidence"`
	imageData        map[string]llm.ImageData
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
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("review material cannot include symbolic link %s", rel)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported review source %s", rel)
		}
		// Refuse an incomplete snapshot rather than silently omitting artifacts.
		if info.Size() > 64<<20 || int64(total)+info.Size() > 128<<20 {
			return errors.New("review source material exceeds 128 MiB snapshot budget")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		total += len(raw)
		file := reviewSourceFile{Hash: hashBytes(raw), Size: len(raw), Binary: !utf8.Valid(raw) || strings.IndexByte(string(raw), 0) >= 0}
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
		if existed && exists && old.Hash == next.Hash {
			continue
		}
		change := ReviewFileChange{Path: path, Kind: "modified", BeforeHash: old.Hash, AfterHash: next.Hash, Binary: old.Binary || next.Binary}
		if !existed {
			change.Kind = "added"
		}
		if !exists {
			change.Kind = "deleted"
		}
		if !change.Binary {
			diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{A: difflib.SplitLines(old.Content), B: difflib.SplitLines(next.Content), FromFile: "run-start/" + path, ToFile: "current/" + path, Context: 3})
			if err != nil {
				return nil, err
			}
			change.Diff = diff
		}
		changes = append(changes, change)
	}
	return changes, nil
}

// buildReviewMaterial fails on incomplete evidence; the runtime supplies a
// backend renderer through prepareReviewMaterial to fill missing derived images.
func buildReviewMaterial(ctx context.Context, state *RunState, demand string) (ReviewMaterial, []llm.ContentPart, error) {
	return prepareReviewMaterial(ctx, state, demand, nil)
}

type reviewEvidenceRenderer func(context.Context, string, model.RunScope) (ToolResult, error)

func prepareReviewMaterial(ctx context.Context, state *RunState, demand string, render reviewEvidenceRenderer) (ReviewMaterial, []llm.ContentPart, error) {
	scope := state.scope
	if scope.Source.Kind == "" {
		scope = state.pack.Command.Scope
	}
	material := ReviewMaterial{UserInstructions: append([]ReviewInstruction{}, state.reviewInstructions...), Demand: demand, Scope: scope, Options: state.pack.Command.Options, Pages: []ReviewPage{}, TaskChanges: []ReviewFileChange{}, Sources: []ReviewSource{}, Resources: map[string]bool{}, ImageEvidence: []ReviewImageEvidence{}, imageData: map[string]llm.ImageData{}}
	if state.reviewBaselineError != "" {
		return material, nil, errors.New(state.reviewBaselineError)
	}
	raw, err := os.ReadFile(reviewBaselinePath(state.projectDir, state.runID))
	if err != nil {
		return material, nil, err
	}
	var baseline map[string]reviewSourceFile
	if err := json.Unmarshal(raw, &baseline); err != nil {
		return material, nil, err
	}
	current, err := reviewSourceFiles(ctx, state.projectDir)
	if err != nil {
		return material, nil, err
	}
	material.SourceHash = hashCheckpointValue(current)
	material.Changes, err = reviewChanges(baseline, current)
	if err != nil {
		return material, nil, err
	}
	// Keep exact current source, including unchanged dependencies and full local
	// diff hunks. Capacity failure belongs to the runtime, never silent pruning.
	paths := make([]string, 0, len(current))
	for path := range current {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		material.Sources = append(material.Sources, ReviewSource{Path: path, reviewSourceFile: current[path]})
	}
	for _, change := range material.Changes {
		id := strings.TrimSuffix(filepath.Base(change.Path), ".html")
		isSlideHTML := filepath.Ext(change.Path) == ".html" && change.Path == model.SlideHTMLPath(id) && strings.HasPrefix(id, "sli_")
		if !isSlideHTML || scope.Source.Kind == model.ScopeAllPages || scope.ContainsSlide(id) {
			material.TaskChanges = append(material.TaskChanges, change)
		}
	}
	for _, path := range []string{".manifest.json", ".outline.json", ".design.json", model.SpecCollectionPath} {
		_, material.Resources[path] = current[path]
	}
	outline, err := currentOutline(state.pack, state.tx)
	if err != nil {
		return material, nil, fmt.Errorf("read review page directory: %w", err)
	}
	// Reading/deriving evidence may cover dependencies outside the editing scope.
	// This local scope authorizes only the backend renderer, never artifact edits.
	renderScope := scope
	renderScope.SlideIDs = []string{}
	locations := spec.FlattenOutline(outline)
	for _, loc := range locations {
		renderScope.SlideIDs = append(renderScope.SlideIDs, loc.Slide.ID)
	}
	parts := []llm.ContentPart{}
	imageBytes, renders := 0, 0
	addImage := func(evidence ReviewImageEvidence, data llm.ImageData, label string) error {
		imageBytes += len(data.Bytes)
		if imageBytes > 128<<20 {
			return errors.New("review images exceed 128 MiB evidence budget; no image was omitted")
		}
		evidence.Hash = hashBytes(data.Bytes)
		material.ImageEvidence = append(material.ImageEvidence, evidence)
		material.imageData[evidence.ImageRef] = data
		parts = append(parts, llm.ContentPart{Type: "text", Text: label}, llm.ContentPart{Type: "image", ImageRef: evidence.ImageRef, MIMEType: data.MIMEType, Detail: "high"})
		return nil
	}
	for i, location := range locations {
		if err := ctx.Err(); err != nil {
			return material, nil, err
		}
		id := location.Slide.ID
		html, exists := current[model.SlideHTMLPath(id)]
		page := ReviewPage{SlideID: id, Number: i + 1, HTMLExists: exists}
		if !exists {
			material.Pages = append(material.Pages, page)
			continue
		}
		proof, err := currentRenderProof(state.pack, state.projectDir, state.tx, id, html.Hash)
		if err != nil {
			return material, nil, fmt.Errorf("read required render sources for %s: %w", id, err)
		}
		dependency := proof.SourceHash + ":" + proof.FrameContextHash
		entry, loadErr := renderimage.Latest(state.projectDir, state.pack.Project.ID, id)
		var pixels []byte
		if loadErr == nil && entry.SourceHash == html.Hash && entry.DependencyHash == dependency {
			_, pixels, loadErr = renderimage.Read(ctx, state.projectDir, state.pack.Project.ID, entry.ImageRef())
		} else {
			loadErr = renderimage.ErrUnavailable
		}
		if loadErr != nil {
			if render == nil {
				return material, nil, fmt.Errorf("required review screenshot is missing or stale for %s", id)
			}
			if renders >= reviewMaxEvidenceRenders {
				return material, nil, errors.New("review evidence render budget exhausted")
			}
			renders++
			result, err := render(ctx, id, renderScope)
			if err != nil {
				return material, nil, err
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
				return material, nil, fmt.Errorf("prepare review screenshot for %s failed (%s)", id, result.Code)
			}
			if result.Data != nil {
				page.Diagnostics, _ = result.Data["model_diagnostics"].(map[string]any)
			}
			entry, loadErr = renderimage.Latest(state.projectDir, state.pack.Project.ID, id)
			if loadErr != nil || entry.SourceHash != html.Hash || entry.DependencyHash != dependency {
				return material, nil, fmt.Errorf("review screenshot for %s does not match the source snapshot", id)
			}
			_, pixels, err = renderimage.Read(ctx, state.projectDir, state.pack.Project.ID, entry.ImageRef())
			if err != nil {
				return material, nil, fmt.Errorf("read required review screenshot for %s: %w", id, err)
			}
		}
		page.Render = &RenderedImageContext{SlideID: id, ImagePath: entry.ImagePath(), SourceHash: entry.SourceHash, RenderedAt: entry.RenderedAt}
		page.RenderDependencyHash = dependency
		if err := addImage(ReviewImageEvidence{ImageRef: entry.ImageRef(), SlideID: id}, llm.ImageData{Bytes: pixels, MIMEType: "image/png"}, fmt.Sprintf("Current screenshot for page %d (slide_id: %s)", i+1, id)); err != nil {
			return material, nil, err
		}
		material.Pages = append(material.Pages, page)
	}
	attachments := append([]model.AttachmentReference{}, state.pack.Command.Attachments...)
	for _, instruction := range material.UserInstructions {
		attachments = append(attachments, instruction.Attachments...)
	}
	for _, image := range state.readImages {
		if image.AttachmentID != "" {
			attachments = append(attachments, model.AttachmentReference{ID: image.AttachmentID})
		}
	}
	seen := map[string]bool{}
	for _, ref := range attachments {
		if seen[ref.ID] {
			continue
		}
		seen[ref.ID] = true
		meta, pixels, err := attachment.Read(ctx, state.projectDir, state.pack.Project.ID, ref.ID, "original")
		if err != nil {
			return material, nil, fmt.Errorf("read required review attachment %s: %w", ref.ID, err)
		}
		file, exists := current[meta.OriginalPath()]
		if !exists || file.Hash != hashBytes(pixels) {
			return material, nil, errors.New("review attachment changed during evidence preparation")
		}
		name := ref.OriginalName
		if name == "" {
			name = meta.OriginalName
		}
		if err := addImage(ReviewImageEvidence{ImageRef: meta.ImageRef(state.pack.Project.ID, "original"), AttachmentID: ref.ID}, llm.ImageData{Bytes: pixels, MIMEType: meta.MediaType}, "Uploaded reference attachment_id: "+ref.ID+" ("+name+")"); err != nil {
			return material, nil, err
		}
	}
	material.EvidenceVersion = hashCheckpointValue(struct {
		SourceHash   string
		Images       []ReviewImageEvidence
		Instructions []ReviewInstruction
		Demand       string
		Scope        model.RunScope
	}{material.SourceHash, material.ImageEvidence, material.UserInstructions, demand, scope})
	if err := validateReviewEvidence(ctx, state, material); err != nil {
		return material, nil, err
	}
	return material, parts, nil
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
	if hashCheckpointValue(current) != material.SourceHash {
		return errors.New("artifacts changed during review; the assessment no longer applies to the current content")
	}
	for _, page := range material.Pages {
		if page.Render == nil {
			continue
		}
		proof, err := currentRenderProof(state.pack, state.projectDir, state.tx, page.SlideID, page.Render.SourceHash)
		if err != nil {
			return err
		}
		if proof.SourceHash+":"+proof.FrameContextHash != page.RenderDependencyHash {
			return errors.New("render dependencies changed during review; the assessment is stale")
		}
	}
	return ctx.Err()
}
