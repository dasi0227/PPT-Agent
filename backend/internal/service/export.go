package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	presentationexport "github.com/dasi0227/PPT-Agent/backend/internal/export"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

var ErrExportInvalid = errors.New("service: invalid export request")

type ExportService struct {
	store   store.Store
	locks   *run.LockManager
	themes  *ThemeService
	manager *presentationexport.Manager
}

func NewExportService(s store.Store, locks *run.LockManager, themes *ThemeService, manager *presentationexport.Manager) *ExportService {
	return &ExportService{store: s, locks: locks, themes: themes, manager: manager}
}

func (svc *ExportService) Manager() *presentationexport.Manager { return svc.manager }

func (svc *ExportService) Start(ctx context.Context, projectID string, format presentationexport.Format, requestID string) (*presentationexport.Operation, error) {
	projectID, requestID = strings.TrimSpace(projectID), strings.TrimSpace(requestID)
	if projectID == "" || requestID == "" || !format.Valid() {
		return nil, ErrExportInvalid
	}
	if existing, ok := svc.manager.Existing(projectID, requestID); ok {
		return existing, nil
	}
	if active, checkErr := svc.store.HasActiveRun(ctx, projectID); checkErr != nil {
		return nil, checkErr
	} else if active {
		return nil, ErrRunActive
	}
	if active, checkErr := svc.store.HasActiveGitCommit(ctx, projectID); checkErr != nil {
		return nil, checkErr
	} else if active {
		return nil, ErrGitCommitActive
	}
	if svc.manager.Active(projectID) {
		return nil, presentationexport.ErrAlreadyActive
	}
	release, lockErr := svc.locks.Acquire(ctx, projectID, 5*time.Second)
	if lockErr != nil {
		return nil, ErrRunActive
	}
	defer release()
	if existing, exists := svc.manager.Existing(projectID, requestID); exists {
		return existing, nil
	}
	if active, checkErr := svc.store.HasActiveRun(ctx, projectID); checkErr != nil {
		return nil, checkErr
	} else if active {
		return nil, ErrRunActive
	}
	if active, checkErr := svc.store.HasActiveGitCommit(ctx, projectID); checkErr != nil {
		return nil, checkErr
	} else if active {
		return nil, ErrGitCommitActive
	}
	if svc.manager.Active(projectID) {
		return nil, presentationexport.ErrAlreadyActive
	}
	project, err := svc.store.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	var design spec.Design
	designRaw, err := os.ReadFile(filepath.Join(project.WorkDir, "design.json"))
	if err != nil || json.Unmarshal(designRaw, &design) != nil || strings.TrimSpace(design.Theme) == "" {
		return nil, &presentationexport.SnapshotError{Code: "EXPORT_THEME_UNAVAILABLE", Message: "导出主题不可用。"}
	}
	themeCSS, err := svc.themes.CSS(design.Theme)
	if err != nil {
		return nil, &presentationexport.SnapshotError{Code: "EXPORT_THEME_UNAVAILABLE", Message: "导出主题不可用。"}
	}
	id := model.MustShortID("exp")
	snapshot, err := presentationexport.CreateSnapshot(ctx, presentationexport.SnapshotInput{ExportID: id, ProjectID: project.ID, ProjectTitle: project.Title, ProjectDir: project.WorkDir, ThemeID: design.Theme, ThemeCSS: themeCSS})
	if err != nil {
		return nil, err
	}
	return svc.manager.Start(id, requestID, format, snapshot)
}

func (svc *ExportService) Get(id string) (*presentationexport.Operation, error) {
	return svc.manager.Get(strings.TrimSpace(id))
}
func (svc *ExportService) Subscribe(id string, after int64) (<-chan presentationexport.Event, func(), error) {
	return svc.manager.Subscribe(strings.TrimSpace(id), after)
}
func (svc *ExportService) Cancel(id string) error         { return svc.manager.Cancel(strings.TrimSpace(id)) }
func (svc *ExportService) CancelProject(projectID string) { svc.manager.CancelProject(projectID) }
