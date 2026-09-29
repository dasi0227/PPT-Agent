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

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/pmezard/go-difflib/difflib"
)

// User instructions are kept separately from the main agent's transcript so
// compaction cannot erase corrections that the reviewer needs to judge artifacts.
type ReviewInstruction struct {
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
	SlideID    string                `json:"slide_id"`
	Number     int                   `json:"number"`
	HTMLExists bool                  `json:"html_exists"`
	Render     *RenderedImageContext `json:"latest_render,omitempty"`
}

type ReviewMaterial struct {
	SourceHash       string              `json:"-"`
	Options          model.RunOptions    `json:"user_options"`
	UserInstructions []ReviewInstruction `json:"user_instructions"`
	Demand           string              `json:"demand"`
	Changes          []ReviewFileChange  `json:"changes"`
	Pages            []ReviewPage        `json:"pages"`
}

func reviewBaselinePath(root, runID string) string {
	return filepath.Join(root, ".runtime", "review-baselines", hashBytes([]byte(runID))+".json")
}

// Match the project history boundary: runtime output, Git internals and saved
// versions are not authored artifacts. Other project files, including assets,
// are included, even when edited through run_command rather than PPT tools.
func reviewSourceFiles(ctx context.Context, root string) (map[string]reviewSourceFile, error) {
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

func buildReviewMaterial(ctx context.Context, state *RunState, demand string) (ReviewMaterial, []llm.ContentPart, error) {
	material := ReviewMaterial{UserInstructions: append([]ReviewInstruction{}, state.reviewInstructions...), Demand: demand, Options: state.pack.Command.Options, Pages: []ReviewPage{}}
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
	outline, err := currentOutline(state.pack, state.tx)
	if err != nil {
		return material, nil, fmt.Errorf("read review page directory: %w", err)
	}
	images := map[string]RenderedImageContext{}
	for _, image := range latestRenderedImages(state.pack, state.projectDir, state.tx) {
		images[image.SlideID] = image
	}
	parts := []llm.ContentPart{}
	for i, location := range spec.FlattenOutline(outline) {
		id := location.Slide.SlideID
		_, exists := current[model.SlideHTMLPath(id)]
		page := ReviewPage{SlideID: id, Number: i + 1, HTMLExists: exists}
		if image, ok := images[id]; ok && exists {
			page.Render = &image
			result := (readImageTool{}).Execute(ctx, DomainToolInput{Args: map[string]any{"image_path": image.ImagePath}, Context: state.pack, ProjectDir: state.projectDir, RunID: state.runID, Session: state.tx})
			if !result.OK {
				return material, nil, fmt.Errorf("could not load existing review screenshot for %s: %s", id, result.Summary)
			}
			parts = append(parts, result.ObservationParts...)
		}
		material.Pages = append(material.Pages, page)
	}
	return material, parts, nil
}
