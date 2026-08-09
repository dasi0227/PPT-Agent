package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

// ProjectService 管理 Project 生命周期与 work_dir 初始化。
type ProjectService struct {
	store    store.Store
	workRoot string
	clock    func() int64
	newID    func() string
}

type CreateProjectParams struct {
	Topic      string
	Brief      string
	SlideCount int
	Language   string
}

func NewProjectService(s store.Store, workRoot WorkRoot) *ProjectService {
	return &ProjectService{store: s, workRoot: string(workRoot), clock: func() int64 { return time.Now().Unix() }, newID: func() string { return model.MustShortID("pro") }}
}

func (svc *ProjectService) CreateProject(ctx context.Context, p CreateProjectParams) (model.Project, error) {
	title := strings.TrimSpace(p.Topic)
	if title == "" {
		return model.Project{}, ErrInvalidProject
	}
	id := svc.newID()
	now := svc.clock()
	workDir := filepath.Join(svc.workRoot, "projects", id)
	proj := model.Project{
		ID:              id,
		Title:           title,
		WorkDir:         workDir,
		Theme:           "swiss-modern",
		Status:          "draft",
		OutlineRevision: 1,
		DesignRevision:  1,
		LayoutVersion:   3,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := svc.initWorkDir(proj, p); err != nil {
		return model.Project{}, err
	}
	if err := svc.store.CreateProject(ctx, proj); err != nil {
		_ = os.RemoveAll(workDir)
		return model.Project{}, err
	}
	return proj, nil
}

func (svc *ProjectService) RenameProject(ctx context.Context, id, title string) (model.Project, error) {
	p, err := svc.store.GetProject(ctx, id)
	if err != nil {
		return model.Project{}, err
	}
	p.Title = title
	p.UpdatedAt = svc.clock()
	if err := svc.store.UpdateProjectTitle(ctx, p.ID, p.Title, p.UpdatedAt); err != nil {
		return model.Project{}, err
	}
	return svc.projectWithFileRevisions(p)
}

func (svc *ProjectService) ListProjects(ctx context.Context) ([]model.Project, error) {
	projects, err := svc.store.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	for index := range projects {
		projects[index], err = svc.projectWithFileRevisions(projects[index])
		if err != nil {
			return nil, err
		}
	}
	return projects, nil
}

func (svc *ProjectService) GetProject(ctx context.Context, id string) (model.Project, error) {
	project, err := svc.store.GetProject(ctx, id)
	if err != nil {
		return model.Project{}, err
	}
	return svc.projectWithFileRevisions(project)
}

func (svc *ProjectService) projectWithFileRevisions(project model.Project) (model.Project, error) {
	var outline spec.Outline
	if err := readJSON(filepath.Join(project.WorkDir, "outline.json"), &outline); err != nil {
		return model.Project{}, err
	}
	project.OutlineRevision = outline.Revision
	var design spec.Design
	if err := readJSON(filepath.Join(project.WorkDir, "design.json"), &design); err != nil {
		return model.Project{}, err
	}
	project.DesignRevision = design.Revision
	project.Theme = design.Theme
	return project, nil
}

func (svc *ProjectService) DeleteProject(ctx context.Context, id string) error {
	proj, err := svc.store.GetProject(ctx, id)
	if err != nil {
		return err
	}
	if err := svc.store.DeleteProject(ctx, id); err != nil {
		return err
	}
	return os.RemoveAll(proj.WorkDir)
}

func (svc *ProjectService) ListSlides(ctx context.Context, projectID string) ([]model.Slide, error) {
	if _, err := svc.store.GetProject(ctx, projectID); err != nil {
		return nil, err
	}
	slides, err := svc.store.ListSlides(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return projectSlidesFromFiles(ctx, svc.store, projectID, slides)
}

func (svc *ProjectService) initWorkDir(proj model.Project, p CreateProjectParams) error {
	sb, err := artifactfs.NewSandbox(svc.workRoot)
	if err != nil {
		return err
	}
	projectRel := filepath.Join("projects", proj.ID)
	for _, rel := range []string{projectRel, filepath.Join(projectRel, "threads"), filepath.Join(projectRel, "common"), filepath.Join(projectRel, "slides")} {
		abs, err := sb.Resolve(rel)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return err
		}
	}
	state := map[string]any{
		"project_id":    proj.ID,
		"title":         proj.Title,
		"current_state": "draft",
		"theme":         proj.Theme,
		"slide_count":   p.SlideCount,
		"language":      p.Language,
		"updated_at":    proj.UpdatedAt,
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := sb.Write(filepath.Join(projectRel, "state.json"), raw); err != nil {
		return err
	}
	outline := spec.Outline{
		SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: proj.ID, Title: proj.Title,
		Goal: firstNonEmpty(p.Brief, proj.Title), Audience: "待明确",
		Language: firstNonEmpty(p.Language, "zh-CN"), Positioning: proj.Title,
		Constraints: spec.Constraints{
			MustInclude: []string{}, MustAvoid: []string{}, StyleLimits: []string{}, ContentLimits: []string{},
		},
		Sections:   []spec.Section{},
		SlideOrder: []string{}, CreatedAt: proj.CreatedAt, UpdatedAt: proj.UpdatedAt,
	}
	if err := sb.Write(filepath.Join(projectRel, "outline.json"), mustJSON(outline)); err != nil {
		return err
	}
	design := defaultDesign(proj.ID, proj.CreatedAt)
	if err := sb.Write(filepath.Join(projectRel, "design.json"), mustJSON(design)); err != nil {
		return err
	}
	baseCSS, err := asset.ReadSeedFile("common/base.css")
	if err != nil {
		return err
	}
	if err := sb.Write(filepath.Join(projectRel, "common", "base.css"), baseCSS); err != nil {
		return err
	}
	return sb.Write(filepath.Join(projectRel, "common", "tokens.css"), spec.DesignTokensCSS(design))
}
