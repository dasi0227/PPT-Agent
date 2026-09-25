package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/projecthistory"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type SlideSourceDocument struct {
	ProjectID      string  `json:"project_id"`
	SlideID        string  `json:"slide_id"`
	Kind           string  `json:"kind"`
	Path           string  `json:"path"`
	Language       string  `json:"language"`
	Content        string  `json:"content"`
	SourceHash     string  `json:"source_hash"`
	ContentHash    *string `json:"content_hash"`
	SceneRevision  int64   `json:"scene_revision"`
	Writable       bool    `json:"writable"`
	ReadonlyReason *string `json:"readonly_reason"`
}

type SlideSourceError struct {
	Code           string
	Message        string
	Pointer        string
	DiagnosticCode string
	Line           int
	Column         int
}

func (e *SlideSourceError) Error() string    { return e.Message }
func sourceError(code, message string) error { return &SlideSourceError{Code: code, Message: message} }

type SlideSourceService struct{ History *projecthistory.Manager }

func NewSlideSourceService(history *projecthistory.Manager) *SlideSourceService {
	return &SlideSourceService{History: history}
}

func (s *SlideSourceService) target(ctx context.Context, projectID, slideID, kind string) (*artifactfs.Sandbox, string, error) {
	var rel string
	switch kind {
	case "spec":
		rel = model.SlideSpecPath(slideID)
	case "html":
		rel = model.SlideHTMLPath(slideID)
	case "manifest", "design":
		rel = kind + ".json"
	default:
		return nil, "", sourceError("SOURCE_REQUEST_INVALID", "文件类型无效")
	}
	projectSource := kind == "manifest" || kind == "design"
	if projectSource != (slideID == "") {
		return nil, "", sourceError("SOURCE_REQUEST_INVALID", "文件类型与资源范围不匹配")
	}
	project, err := s.History.Store.GetProject(ctx, projectID)
	if err != nil {
		return nil, "", sourceError("SOURCE_NOT_FOUND", "项目不存在")
	}
	sandbox, err := artifactfs.NewSandbox(project.WorkDir)
	if err != nil {
		return nil, "", err
	}
	if projectSource {
		return sandbox, rel, nil
	}
	slide, err := s.History.Store.GetSlide(ctx, slideID)
	if err != nil || slide.ProjectID != projectID {
		return nil, "", sourceError("SOURCE_NOT_FOUND", "页面不存在")
	}
	outlineRaw, err := sandbox.Read("outline.json")
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

func (s *SlideSourceService) busy(ctx context.Context, projectID string) (string, error) {
	active, err := s.History.Store.HasActiveRun(ctx, projectID)
	if err != nil {
		return "", err
	}
	if active {
		return "RUN_ACTIVE", nil
	}
	active, err = s.History.Store.HasActiveGitCommit(ctx, projectID)
	if err != nil {
		return "", err
	}
	if active {
		return "GIT_COMMIT_ACTIVE", nil
	}
	if s.History.ExportActive != nil && s.History.ExportActive(projectID) {
		return "HISTORY_BUSY", nil
	}
	return "", nil
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
	busy, err := s.busy(ctx, projectID)
	if err != nil {
		return document, err
	}
	document = SlideSourceDocument{ProjectID: projectID, SlideID: slideID, Kind: kind, Path: rel, Content: string(raw), SourceHash: spec.ContentHash(raw), SceneRevision: state.SceneRevision, Writable: busy == ""}
	if busy != "" {
		document.ReadonlyReason = &busy
	}
	if kind != "html" {
		document.Language = "json"
		if valid, validErr := spec.ParseStrictSourceJSON(raw, kind); validErr == nil {
			hash := spec.ResourceHash(valid)
			document.ContentHash = &hash
		}
	} else {
		document.Language = "html"
		hash := document.SourceHash
		document.ContentHash = &hash
	}
	return document, nil
}

// Save is called while the outer history gate is held. It takes the shared
// project lock only after that gate, matching the existing lock order.
func (s *SlideSourceService) Save(ctx context.Context, projectID, slideID, kind, content, expectedHash string, expectedScene int64, discardRevision int64) (SlideSourceDocument, bool, error) {
	var zero SlideSourceDocument
	if expectedHash == "" || expectedScene < 0 {
		return zero, false, sourceError("SOURCE_REQUEST_INVALID", "缺少保存前置条件")
	}
	release, err := s.History.Locks.Acquire(ctx, projectID, 5*time.Second)
	if err != nil {
		return zero, false, sourceError("HISTORY_BUSY", "项目正忙，请稍后重试")
	}
	defer release()
	if s.History.Switching(projectID) {
		return zero, false, sourceError("HISTORY_BUSY", "项目历史正在切换")
	}
	state, err := s.History.State(projectID)
	if err != nil {
		return zero, false, err
	}
	if state.SceneRevision != expectedScene {
		return zero, false, sourceError("SOURCE_SCENE_CHANGED", "项目历史现场已改变")
	}
	busy, err := s.busy(ctx, projectID)
	if err != nil {
		return zero, false, err
	}
	if busy != "" {
		return zero, false, sourceError(busy, "项目存在冲突操作，暂时不能保存")
	}
	sandbox, rel, err := s.target(ctx, projectID, slideID, kind)
	if err != nil {
		return zero, false, err
	}
	path, err := sandbox.Resolve(rel)
	if err != nil {
		return zero, false, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return zero, false, sourceError("SOURCE_NOT_FOUND", "源文件不存在")
	}
	if err != nil {
		return zero, false, err
	}
	if !info.Mode().IsRegular() {
		return zero, false, sourceError("SOURCE_NOT_FOUND", "源文件不可编辑")
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return zero, false, err
	}
	if spec.ContentHash(current) != expectedHash {
		return zero, false, sourceError("CONTENT_CONFLICT", "源文件已更新")
	}
	var candidate []byte
	if kind != "html" {
		next, parseErr := spec.ParseStrictSourceJSON([]byte(content), kind)
		if parseErr != nil {
			return zero, false, sourceValidationError([]byte(content), parseErr)
		}
		candidate, err = json.MarshalIndent(next, "", "  ")
		if err != nil {
			return zero, false, err
		}
		candidate = append(candidate, '\n')
	} else {
		candidate = []byte(content)
		if err = spec.ValidateSlideHTML(candidate); err != nil {
			return zero, false, sourceError("SOURCE_VALIDATION_FAILED", err.Error())
		}
	}
	if bytes.Equal(candidate, current) {
		document, readErr := s.Read(ctx, projectID, slideID, kind)
		return document, false, readErr
	}
	if state.Latest != "" && discardRevision == 0 {
		return zero, false, &SlideSourceError{Code: "HISTORY_CONFIRM_REQUIRED", Message: "继续创作将丢弃原来的后续历史，无法再恢复到最新现场。", Pointer: strconv.FormatInt(state.Revision, 10)}
	}
	finish, err := s.History.BeginMutation(ctx, projectID, discardRevision)
	if err != nil {
		return zero, false, err
	}
	if err = replaceExistingSource(path, info.Mode().Perm(), candidate); err != nil {
		_ = finish(false)
		return zero, false, err
	}
	if err = finish(true); err != nil {
		return zero, false, err
	}
	document, err := s.Read(ctx, projectID, slideID, kind)
	return document, true, err
}

func sourceValidationError(raw []byte, err error) error {
	issue := &SlideSourceError{Code: "SOURCE_VALIDATION_FAILED", DiagnosticCode: "SOURCE_INVALID", Message: err.Error()}
	if strings.HasPrefix(issue.Message, "duplicate JSON key at ") {
		issue.DiagnosticCode = "JSON_DUPLICATE_KEY"
		issue.Pointer = strings.TrimPrefix(issue.Message, "duplicate JSON key at ")
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		issue.DiagnosticCode = "JSON_SYNTAX"
		offset := int(syntax.Offset)
		if offset > 0 && offset <= len(raw) {
			prefix := raw[:offset-1]
			issue.Line = bytes.Count(prefix, []byte{'\n'}) + 1
			issue.Column = len(prefix) - bytes.LastIndexByte(prefix, '\n')
		}
	}
	return issue
}

func replaceExistingSource(path string, mode fs.FileMode, content []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".source-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(mode); err == nil {
		_, err = file.Write(content)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	// Another process may have removed the file while the candidate was being
	// written. This endpoint updates existing author files only.
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		if err != nil {
			return err
		}
		return sourceError("SOURCE_NOT_FOUND", "源文件不可编辑")
	}
	if err = os.Rename(file.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
