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
	"github.com/dasi0227/PPT-Agent/backend/internal/branding"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
	"github.com/dasi0227/PPT-Agent/backend/internal/slidepreview"
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
	if err := pptmutation.CheckProjectWritable(project.WorkDir); err != nil {
		return spec.ProjectContentSnapshot{}, pptmutation.Result{}, err
	}
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
	metaRaw, err := read(".meta.json")
	metaExists := err == nil
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
	constraintsRaw, err := read(".constraints.json")
	constraintsExists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return spec.ProjectContentSnapshot{}, err
	}
	parsed, err := spec.ParseStrictSourceJSON(outlineRaw, "outline")
	if err != nil {
		return spec.ProjectContentSnapshot{}, err
	}
	outline := *parsed.(*spec.Outline)
	out := spec.ProjectContentSnapshot{ProjectID: projectID, ProjectTitle: project.Title, Theme: project.Theme, Decorations: project.Decorations, Logo: branding.PreviewImage(projectID, project.Logo), Hashes: map[string]string{}, Outline: outline, SlidesByID: map[string]spec.SlideContent{}}
	if outlineExists {
		out.Hashes["outline"] = spec.ResourceHash(outline)
	}
	if metaExists {
		parsed, err := spec.ParseStrictSourceJSON(metaRaw, "meta")
		if err != nil {
			return spec.ProjectContentSnapshot{}, err
		}
		out.Meta = parsed.(*spec.Meta)
		out.Hashes["meta"] = spec.ResourceHash(out.Meta)
	}
	if constraintsExists {
		parsed, err := spec.ParseStrictSourceJSON(constraintsRaw, "constraints")
		if err != nil {
			return spec.ProjectContentSnapshot{}, err
		}
		out.Constraints = parsed.(*spec.Constraints)
		out.Hashes["constraints"] = spec.ResourceHash(out.Constraints)
	}
	out.Appearance, err = runtimeassets.ProjectAppearance(project.WorkDir, project.Theme)
	if err != nil {
		out.ThemeError = err.Error()
	} else {
		out.Hashes["appearance"] = out.Appearance.Hash
	}
	for _, loc := range spec.FlattenOutline(outline) {
		id := loc.Slide.ID
		content := spec.SlideContent{SpecState: "pending", HTMLState: "missing"}
		specRaw, specErr := spec.ReadSlideSpec(read, id)
		var slide spec.SlideSpec
		if specErr == nil {
			if err := json.Unmarshal(specRaw, &slide); err != nil {
				return spec.ProjectContentSnapshot{}, err
			}
			content.Spec = &slide
			content.SpecState = "ready"
			out.Hashes["spec:"+id] = spec.ResourceHash(slide)
		} else if errors.Is(specErr, spec.ErrInvalid) {
			content.SpecState = "invalid"
		} else if !errors.Is(specErr, fs.ErrNotExist) {
			return spec.ProjectContentSnapshot{}, specErr
		}
		htmlRaw, htmlErr := read(model.SlideHTMLPath(id))
		if htmlErr == nil {
			content.HTMLHash = spec.ContentHash(htmlRaw)
		}
		if htmlErr == nil {
			content.HTMLState = "available"
			if out.Appearance != nil {
				meta := spec.Meta{Title: project.Title}
				if out.Meta != nil {
					meta = *out.Meta
				}
				if frame, ok := spec.BuildRuntimeFrame(meta, outline, project.Decorations, out.Logo, id, slide, out.Appearance); ok {
					out.Hashes["thumbnail:"+id] = slidepreview.Version(project.WorkDir, id, slidepreview.SourceKey(content.HTMLHash, frame))
				}
			}
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
