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
	releaseMutation := pptmutation.LockProject(project.WorkDir)
	defer releaseMutation()
	sandbox, err := artifactfs.NewSandbox(project.WorkDir)
	if err != nil {
		return spec.ProjectContentSnapshot{}, pptmutation.Result{}, err
	}
	buffer := pptmutation.NewBuffer(sandbox)
	engine := pptmutation.Service{Workspace: buffer}
	req.ProjectTitle = project.Title
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
	snapshot, err := s.snapshot(ctx, projectID)
	return snapshot, result, err
}

func (s *PPTMutationService) Snapshot(ctx context.Context, projectID string) (spec.ProjectContentSnapshot, error) {
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return spec.ProjectContentSnapshot{}, err
	}
	release := pptmutation.ReadLockProject(project.WorkDir)
	defer release()
	return s.snapshot(ctx, projectID)
}

func (s *PPTMutationService) snapshot(ctx context.Context, projectID string) (spec.ProjectContentSnapshot, error) {
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return spec.ProjectContentSnapshot{}, err
	}
	read := func(path string) ([]byte, error) {
		return os.ReadFile(filepath.Join(project.WorkDir, filepath.FromSlash(path)))
	}
	manifestRaw, err := read(".manifest.json")
	manifestExists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return spec.ProjectContentSnapshot{}, err
	}
	outlineRaw, err := read(".outline.json")
	outlineExists := err == nil
	if errors.Is(err, fs.ErrNotExist) {
		outlineRaw = []byte(`{"sections":[]}`)
		err = nil
	}
	if err != nil {
		return spec.ProjectContentSnapshot{}, err
	}
	designRaw, err := read(".design.json")
	designExists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return spec.ProjectContentSnapshot{}, err
	}
	for kind, raw := range map[string][]byte{"outline": outlineRaw} {
		if _, err := spec.ParseStrictSourceJSON(raw, kind); err != nil {
			return spec.ProjectContentSnapshot{}, err
		}
	}
	if manifestExists {
		if _, err := spec.ParseStrictSourceJSON(manifestRaw, "manifest"); err != nil {
			return spec.ProjectContentSnapshot{}, err
		}
	}
	if designExists {
		if _, err := spec.ParseStrictSourceJSON(designRaw, "design"); err != nil {
			return spec.ProjectContentSnapshot{}, err
		}
	}
	var outline spec.Outline
	if json.Unmarshal(outlineRaw, &outline) != nil {
		return spec.ProjectContentSnapshot{}, errors.New("project content is invalid")
	}
	out := spec.ProjectContentSnapshot{ProjectID: projectID, ProjectTitle: project.Title, Theme: project.Theme, Hashes: map[string]string{}, Outline: outline, SlidesByID: map[string]spec.SlideContent{}}
	if outlineExists {
		out.Hashes["outline"] = spec.ResourceHash(outline)
	}
	if manifestExists {
		var manifest spec.Manifest
		if json.Unmarshal(manifestRaw, &manifest) != nil {
			return spec.ProjectContentSnapshot{}, errors.New("project content is invalid")
		}
		out.Manifest = &manifest
		out.Hashes["manifest"] = spec.ResourceHash(manifest)
	}
	if designExists {
		var design spec.Design
		if json.Unmarshal(designRaw, &design) != nil {
			return spec.ProjectContentSnapshot{}, errors.New("project content is invalid")
		}
		out.Design = &design
		out.Hashes["design"] = spec.ResourceHash(design)
	}
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
		id := loc.Slide.ID
		content := spec.SlideContent{SpecState: "pending", HTMLState: "missing"}
		specRaw, exists := entries[id]
		var slide spec.SlideSpec
		if exists {
			if err := json.Unmarshal(specRaw, &slide); err != nil {
				return spec.ProjectContentSnapshot{}, err
			}
			content.Spec = &slide
			content.SpecState = "ready"
			out.Hashes["spec:"+id] = spec.ResourceHash(slide)
		}
		htmlRaw, htmlErr := read(model.SlideHTMLPath(id))
		if htmlErr == nil {
			content.HTMLHash = spec.ContentHash(htmlRaw)
		}
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
	outline := spec.Outline{Sections: []spec.Section{}}
	if err := readJSON(filepath.Join(workDir, ".outline.json"), &outline); err != nil && !errors.Is(err, fs.ErrNotExist) {
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
		slide := byID[loc.Slide.ID]
		slide.ID = loc.Slide.ID
		slide.ProjectID = projectID
		next = append(next, slide)
	}
	return s.store.ReplaceSlides(ctx, projectID, next)
}
