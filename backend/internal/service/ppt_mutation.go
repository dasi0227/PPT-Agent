package service

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

type PPTMutationService struct{ store store.Store }

func NewPPTMutationService(st store.Store) *PPTMutationService { return &PPTMutationService{store: st} }

func (s *PPTMutationService) Apply(ctx context.Context, projectID string, req pptmutation.Request) (spec.ProjectContentSnapshot, pptmutation.Result, error) {
	if active, err := s.store.HasActiveRun(ctx, projectID); err != nil {
		return spec.ProjectContentSnapshot{}, pptmutation.Result{}, err
	} else if active {
		return spec.ProjectContentSnapshot{}, pptmutation.Result{}, ErrRunActive
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
	engine := pptmutation.Service{Workspace: buffer, ProjectID: projectID, ValidateHTML: func(raw []byte) error {
		if ok, reason := designsystem.LintSlideResult(raw); !ok {
			return errors.New(reason)
		}
		return nil
	}}
	result, err := engine.Apply(req)
	if err != nil {
		return spec.ProjectContentSnapshot{}, result, err
	}
	if req.Op == "design.write" || req.Op == "design.patch" {
		var design spec.Design
		raw, readErr := buffer.Read("design.json")
		if readErr != nil {
			return spec.ProjectContentSnapshot{}, result, readErr
		}
		if err = jsonUnmarshal(raw, &design); err != nil {
			return spec.ProjectContentSnapshot{}, result, err
		}
		_ = buffer.Write("common/tokens.css", spec.DesignTokensCSS(design))
	}
	if err = buffer.Commit(); err != nil {
		return spec.ProjectContentSnapshot{}, result, err
	}
	if err = s.syncSlideIdentities(ctx, projectID, project.WorkDir); err != nil {
		return spec.ProjectContentSnapshot{}, result, err
	}
	snapshot, err := s.Snapshot(ctx, projectID)
	return snapshot, result, err
}

func (s *PPTMutationService) Snapshot(ctx context.Context, projectID string) (spec.ProjectContentSnapshot, error) {
	view, err := NewSpecService(s.store).EnsureProject(ctx, projectID)
	if err != nil {
		return spec.ProjectContentSnapshot{}, err
	}
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return spec.ProjectContentSnapshot{}, err
	}
	out := spec.ProjectContentSnapshot{Deck: view.Deck, Outline: view.Outline, Design: view.Design, SlidesByID: map[string]spec.SlideContent{}}
	for _, loc := range spec.FlattenOutline(view.Outline) {
		id := loc.Slide.SlideID
		content := spec.SlideContent{SpecState: "pending", HTMLState: "not_materialized"}
		if slide, ok := view.SlideSpecs[id]; ok {
			copy := slide
			content.Spec = &copy
			content.SpecState = "ready"
		}
		if state, ok := view.States[id]; ok {
			content.HTMLState = state.State
		}
		if record, readErr := spec.ReadMaterialization(filepath.Join(project.WorkDir, filepath.FromSlash(model.SlideMaterializationPath(id)))); readErr == nil {
			content.Materialization = &record
			content.HTMLRevision = record.Artifact.Revision
		}
		out.SlidesByID[id] = content
	}
	return out, nil
}

func (s *PPTMutationService) syncSlideIdentities(ctx context.Context, projectID, workDir string) error {
	var outline spec.Outline
	if err := readJSON(filepath.Join(workDir, "outline.json"), &outline); err != nil {
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

// Kept local to avoid exposing encoding details from the mutation package.
func jsonUnmarshal(raw []byte, out any) error { return json.Unmarshal(raw, out) }
