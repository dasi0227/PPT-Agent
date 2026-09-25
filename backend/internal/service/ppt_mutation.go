package service

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type PPTMutationService struct {
	store store.Store
	locks *run.LockManager
}

func NewPPTMutationService(st store.Store) *PPTMutationService { return &PPTMutationService{store: st} }
func NewPPTMutationServiceWithLocks(st store.Store, locks *run.LockManager) *PPTMutationService {
	return &PPTMutationService{store: st, locks: locks}
}

func (s *PPTMutationService) Apply(ctx context.Context, projectID string, req pptmutation.Request) (spec.ProjectContentSnapshot, pptmutation.Result, error) {
	if active, err := s.store.HasActiveRun(ctx, projectID); err != nil {
		return spec.ProjectContentSnapshot{}, pptmutation.Result{}, err
	} else if active {
		return spec.ProjectContentSnapshot{}, pptmutation.Result{}, ErrRunActive
	}
	if active, err := s.store.HasActiveGitCommit(ctx, projectID); err != nil {
		return spec.ProjectContentSnapshot{}, pptmutation.Result{}, err
	} else if active {
		return spec.ProjectContentSnapshot{}, pptmutation.Result{}, ErrGitCommitActive
	}
	release := func() {}
	var lockErr error
	if s.locks != nil {
		release, lockErr = s.locks.Acquire(ctx, projectID, 5*time.Second)
		if lockErr != nil {
			return spec.ProjectContentSnapshot{}, pptmutation.Result{}, ErrRunActive
		}
	}
	defer release()
	if active, err := s.store.HasActiveRun(ctx, projectID); err != nil {
		return spec.ProjectContentSnapshot{}, pptmutation.Result{}, err
	} else if active {
		return spec.ProjectContentSnapshot{}, pptmutation.Result{}, ErrRunActive
	}
	if active, err := s.store.HasActiveGitCommit(ctx, projectID); err != nil {
		return spec.ProjectContentSnapshot{}, pptmutation.Result{}, err
	} else if active {
		return spec.ProjectContentSnapshot{}, pptmutation.Result{}, ErrGitCommitActive
	}
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return spec.ProjectContentSnapshot{}, pptmutation.Result{}, err
	}
	sandbox, err := artifactfs.NewSandbox(project.WorkDir)
	if err != nil {
		return spec.ProjectContentSnapshot{}, pptmutation.Result{}, err
	}
	buffer := pptmutation.NewBuffer(sandbox)
	engine := pptmutation.Service{Workspace: buffer}
	result, err := engine.Apply(req)
	if err != nil {
		return spec.ProjectContentSnapshot{}, result, err
	}
	if err = buffer.Commit(); err != nil {
		return spec.ProjectContentSnapshot{}, result, err
	}
	if err = s.syncSlideIdentities(ctx, projectID, project.WorkDir); err != nil {
		return spec.ProjectContentSnapshot{}, result, errors.Join(err, buffer.Rollback())
	}
	snapshot, err := s.Snapshot(ctx, projectID)
	return snapshot, result, err
}

func (s *PPTMutationService) Snapshot(ctx context.Context, projectID string) (spec.ProjectContentSnapshot, error) {
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return spec.ProjectContentSnapshot{}, err
	}
	read := func(path string) ([]byte, error) {
		if session := workflow.ActiveRunSession(project.WorkDir); session != nil {
			return session.ReadPath(path)
		}
		return os.ReadFile(filepath.Join(project.WorkDir, filepath.FromSlash(path)))
	}
	manifestRaw, err := read(".manifest.json")
	if err != nil {
		return spec.ProjectContentSnapshot{}, err
	}
	outlineRaw, err := read(".outline.json")
	if err != nil {
		return spec.ProjectContentSnapshot{}, err
	}
	designRaw, err := read(".design.json")
	if err != nil {
		return spec.ProjectContentSnapshot{}, err
	}
	for kind, raw := range map[string][]byte{"manifest": manifestRaw, "outline": outlineRaw, "design": designRaw} {
		if _, err := spec.ParseStrictSourceJSON(raw, kind); err != nil {
			return spec.ProjectContentSnapshot{}, err
		}
	}
	var manifest spec.Manifest
	var outline spec.Outline
	var design spec.Design
	if json.Unmarshal(manifestRaw, &manifest) != nil || json.Unmarshal(outlineRaw, &outline) != nil || json.Unmarshal(designRaw, &design) != nil {
		return spec.ProjectContentSnapshot{}, errors.New("project content is invalid")
	}
	out := spec.ProjectContentSnapshot{ProjectID: projectID, Theme: project.Theme, Hashes: map[string]string{"manifest": spec.ResourceHash(manifest), "outline": spec.ResourceHash(outline), "design": spec.ResourceHash(design)}, Manifest: manifest, Outline: outline, Design: design, SlidesByID: map[string]spec.SlideContent{}}
	out.Appearance, err = runtimeassets.ProjectAppearance(project.WorkDir, project.Theme)
	if err != nil {
		out.ThemeError = err.Error()
	} else {
		out.Hashes["appearance"] = out.Appearance.Hash
	}
	entries, err := spec.ReadCollection(read)
	if err != nil {
		return spec.ProjectContentSnapshot{}, err
	}
	for _, loc := range spec.FlattenOutline(outline) {
		id := loc.Slide.SlideID
		content := spec.SlideContent{SpecState: "pending", HTMLState: "missing"}
		specRaw, exists := entries[id]
		var slide spec.SlideSpec
		if exists {
			if err := json.Unmarshal(specRaw, &slide); err != nil {
				return spec.ProjectContentSnapshot{}, err
			}
			content.Spec = &slide
			content.SpecState = "ready"
		}
		htmlRaw, htmlErr := read(model.SlideHTMLPath(id))
		if htmlErr == nil {
			content.HTMLHash = spec.ContentHash(htmlRaw)
		}
		out.Hashes["spec:"+id] = spec.ResourceHash(slide)
		if htmlErr == nil {
			content.HTMLState = "available"
		} else if !errors.Is(htmlErr, fs.ErrNotExist) {
			return spec.ProjectContentSnapshot{}, htmlErr
		}

		out.SlidesByID[id] = content
	}
	return out, nil
}

func (s *PPTMutationService) syncSlideIdentities(ctx context.Context, projectID, workDir string) error {
	var outline spec.Outline
	if err := readJSON(filepath.Join(workDir, ".outline.json"), &outline); err != nil {
		return err
	}
	existing, err := s.store.ListSlides(ctx, projectID)
	if err != nil {
		return err
	}
	byID := map[string]model.Slide{}
	for _, slide := range existing {
		byID[slide.ID] = slide
	}
	next := make([]model.Slide, 0, len(spec.FlattenOutline(outline)))
	for _, loc := range spec.FlattenOutline(outline) {
		slide := byID[loc.Slide.SlideID]
		slide.ID = loc.Slide.SlideID
		slide.ProjectID = projectID
		next = append(next, slide)
	}
	return s.store.ReplaceSlides(ctx, projectID, next)
}
