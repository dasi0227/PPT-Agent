package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
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
	return &ProjectService{store: s, workRoot: string(workRoot), clock: func() int64 { return time.Now().Unix() }, newID: uuid.NewString}
}

func (svc *ProjectService) CreateProject(ctx context.Context, p CreateProjectParams) (model.Project, error) {
	title := strings.TrimSpace(p.Topic)
	if title == "" {
		return model.Project{}, ErrInvalidProject
	}
	id := svc.newID()
	now := svc.clock()
	workDir := filepath.Join(svc.workRoot, id)
	proj := model.Project{
		ID:         id,
		Title:      title,
		WorkDir:    workDir,
		Theme:      "swiss-modern",
		Status:     "draft",
		DesignPath: "common/tokens.css",
		CreatedAt:  now,
		UpdatedAt:  now,
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

func (svc *ProjectService) ListProjects(ctx context.Context) ([]model.Project, error) {
	return svc.store.ListProjects(ctx)
}

func (svc *ProjectService) GetProject(ctx context.Context, id string) (model.Project, error) {
	return svc.store.GetProject(ctx, id)
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
	return svc.store.ListSlides(ctx, projectID)
}

func (svc *ProjectService) initWorkDir(proj model.Project, p CreateProjectParams) error {
	sb, err := tools.NewSandbox(svc.workRoot)
	if err != nil {
		return err
	}
	for _, rel := range []string{proj.ID, filepath.Join(proj.ID, "threads"), filepath.Join(proj.ID, "common"), filepath.Join(proj.ID, "slides")} {
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
	return sb.Write(filepath.Join(proj.ID, "state.json"), raw)
}
