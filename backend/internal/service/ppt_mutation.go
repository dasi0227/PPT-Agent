package service

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
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
	manifestRaw, err := read("manifest.json")
	if err != nil {
		return spec.ProjectContentSnapshot{}, err
	}
	outlineRaw, err := read("outline.json")
	if err != nil {
		return spec.ProjectContentSnapshot{}, err
	}
	designRaw, err := read("design.json")
	if err != nil {
		return spec.ProjectContentSnapshot{}, err
	}
	var manifest spec.Manifest
	var outline spec.Outline
	var design spec.Design
	if json.Unmarshal(manifestRaw, &manifest) != nil || json.Unmarshal(outlineRaw, &outline) != nil || json.Unmarshal(designRaw, &design) != nil {
		return spec.ProjectContentSnapshot{}, errors.New("project content is invalid")
	}
	out := spec.ProjectContentSnapshot{Manifest: manifest, Outline: outline, Design: design, SlidesByID: map[string]spec.SlideContent{}}
	for _, loc := range spec.FlattenOutline(outline) {
		id := loc.Slide.SlideID
		content := spec.SlideContent{SpecState: "pending", HTMLState: "not_materialized"}
		specRaw, specErr := read(model.SlideSpecPath(id))
		var slide spec.SlideSpec
		if specErr == nil && json.Unmarshal(specRaw, &slide) == nil {
			content.Spec = &slide
			content.SpecState = "ready"
		}
		htmlRaw, htmlErr := read(model.SlideHTMLPath(id))
		materialRaw, materialErr := read(model.SlideMaterializationPath(id))
		var record spec.MaterializationRecord
		var recordPtr *spec.MaterializationRecord
		if materialErr == nil && json.Unmarshal(materialRaw, &record) == nil && spec.ValidateMaterialization(record) == nil {
			recordPtr = &record
			content.Materialization = recordPtr
			content.HTMLRevision = record.Artifact.Revision
		}
		if content.SpecState == "ready" {
			nodeHash := spec.SemanticSlideNodeHash(outline, id)
			content.HTMLState = spec.DeriveMaterializationState(htmlErr == nil, recordPtr, manifest.Revision, nodeHash, slide.Revision, design.Revision, spec.ContentHash(htmlRaw), spec.SourceHash(manifestRaw, nodeHash, specRaw, designRaw), spec.FrameContextHash(manifest, outline, design, id))
		} else if htmlErr == nil {
			content.HTMLState = "unknown"
		}
		if errors.Is(htmlErr, fs.ErrNotExist) {
			content.HTMLState = "not_materialized"
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
