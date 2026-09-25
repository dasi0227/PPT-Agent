package service

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/projecthistory"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type SlideSourceDocument struct {
	ProjectID     string `json:"project_id"`
	SlideID       string `json:"slide_id"`
	Path          string `json:"path"`
	Content       string `json:"content"`
	SourceHash    string `json:"source_hash"`
	SceneRevision int64  `json:"scene_revision"`
}

type SlideSourceError struct{ Code, Message string }

func (e *SlideSourceError) Error() string    { return e.Message }
func sourceError(code, message string) error { return &SlideSourceError{Code: code, Message: message} }

type SlideSourceService struct{ History *projecthistory.Manager }

func NewSlideSourceService(history *projecthistory.Manager) *SlideSourceService {
	return &SlideSourceService{History: history}
}

func (s *SlideSourceService) target(ctx context.Context, projectID, slideID, kind string) (*artifactfs.Sandbox, string, error) {
	if kind != "html" || slideID == "" {
		return nil, "", sourceError("SOURCE_REQUEST_INVALID", "仅支持查看幻灯片 HTML 源码")
	}
	rel := model.SlideHTMLPath(slideID)
	project, err := s.History.Store.GetProject(ctx, projectID)
	if err != nil {
		return nil, "", sourceError("SOURCE_NOT_FOUND", "项目不存在")
	}
	sandbox, err := artifactfs.NewSandbox(project.WorkDir)
	if err != nil {
		return nil, "", err
	}
	slide, err := s.History.Store.GetSlide(ctx, slideID)
	if err != nil || slide.ProjectID != projectID {
		return nil, "", sourceError("SOURCE_NOT_FOUND", "页面不存在")
	}
	outlineRaw, err := sandbox.Read(".outline.json")
	if err != nil {
		return nil, "", err
	}
	var outline spec.Outline
	if err = json.Unmarshal(outlineRaw, &outline); err != nil {
		return nil, "", err
	}
	if _, ok := spec.FindSlide(outline, slideID); !ok {
		return nil, "", sourceError("SOURCE_NOT_FOUND", "页面不在当前目录中")
	}
	return sandbox, rel, nil
}

func (s *SlideSourceService) Read(ctx context.Context, projectID, slideID, kind string) (SlideSourceDocument, error) {
	var document SlideSourceDocument
	if s.History.Switching(projectID) {
		return document, sourceError("HISTORY_BUSY", "项目历史正在切换")
	}
	state, err := s.History.State(projectID)
	if err != nil {
		return document, err
	}
	sandbox, rel, err := s.target(ctx, projectID, slideID, kind)
	if err != nil {
		return document, err
	}
	raw, err := sandbox.Read(rel)
	if errors.Is(err, fs.ErrNotExist) {
		return document, sourceError("SOURCE_NOT_FOUND", "源文件尚未生成")
	}
	if err != nil {
		return document, err
	}
	nextState, err := s.History.State(projectID)
	if err != nil {
		return document, err
	}
	if s.History.Switching(projectID) {
		return document, sourceError("HISTORY_BUSY", "项目历史正在切换")
	}
	if nextState.SceneRevision != state.SceneRevision {
		return document, sourceError("SOURCE_SCENE_CHANGED", "项目历史现场已改变")
	}
	document = SlideSourceDocument{ProjectID: projectID, SlideID: slideID, Path: rel, Content: string(raw), SourceHash: spec.ContentHash(raw), SceneRevision: state.SceneRevision}
	return document, nil
}
