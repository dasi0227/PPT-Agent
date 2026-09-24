package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/projecthistory"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"go.uber.org/zap"
)

func sourceFixture(t *testing.T) (*SlideSourceService, model.Project) {
	t.Helper()
	root := t.TempDir()
	db, closeDB, err := sqlitestore.Open(&config.Config{DBPath: filepath.Join(root, "db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDB)
	store, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	project := model.Project{ID: "pro_source", WorkDir: filepath.Join(root, "projects", "pro_source", "artifacts"), Title: "Source", Status: "draft", Theme: "default", LayoutVersion: 6, CreatedAt: 1, UpdatedAt: 1}
	ctx := context.Background()
	if err := store.CreateProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceSlides(ctx, project.ID, []model.Slide{{ID: "sli_source", ProjectID: project.ID}}); err != nil {
		t.Fatal(err)
	}
	write := func(rel, content string) {
		path := filepath.Join(project.WorkDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("outline.json", `{"sections":[{"id":"sec_source","slides":[{"slide_id":"sli_source","title":"Source","role":"content"}]}]}`)
	write(model.SlideSpecPath("sli_source"), `{"version":"5.0","project_id":"pro_source","slide_id":"sli_source","key_message":"原始","elements":[],"created_at":1,"updated_at":1}`)
	write(model.SlideHTMLPath("sli_source"), `<html><body><section class="slide-stage">原始</section></body></html>`)
	return NewSlideSourceService(projecthistory.New(store, run.NewLockManager(), root)), project
}

func sourceCode(err error) string {
	var typed *SlideSourceError
	if errors.As(err, &typed) {
		return typed.Code
	}
	return ""
}

func TestSlideSourceSaveRequiresExactExistingBaselineAndValidContent(t *testing.T) {
	svc, project := sourceFixture(t)
	ctx := context.Background()
	initial, err := svc.Read(ctx, project.ID, "sli_source", "spec")
	if err != nil || initial.SourceHash != spec.ContentHash([]byte(initial.Content)) || !initial.Writable {
		t.Fatalf("read: %+v %v", initial, err)
	}
	if _, _, err = svc.Save(ctx, project.ID, "sli_source", "spec", initial.Content, "", initial.SceneRevision, 0); sourceCode(err) != "SOURCE_REQUEST_INVALID" {
		t.Fatalf("missing CAS: %v", err)
	}
	if _, _, err = svc.Save(ctx, project.ID, "sli_source", "spec", initial.Content, initial.SourceHash, initial.SceneRevision+1, 0); sourceCode(err) != "SOURCE_SCENE_CHANGED" {
		t.Fatalf("scene: %v", err)
	}
	invalid := strings.Replace(initial.Content, `"key_message":"原始"`, `"key_message":"新","key_message":"覆盖"`, 1)
	if _, _, err = svc.Save(ctx, project.ID, "sli_source", "spec", invalid, initial.SourceHash, initial.SceneRevision, 0); sourceCode(err) != "SOURCE_VALIDATION_FAILED" {
		t.Fatalf("duplicate: %v", err)
	}
	immutable := strings.Replace(initial.Content, `"slide_id":"sli_source"`, `"slide_id":"sli_other"`, 1)
	if _, _, err = svc.Save(ctx, project.ID, "sli_source", "spec", immutable, initial.SourceHash, initial.SceneRevision, 0); sourceCode(err) != "SOURCE_VALIDATION_FAILED" {
		t.Fatalf("immutable field: %v", err)
	}
	formatted, changedBytes, err := svc.Save(ctx, project.ID, "sli_source", "spec", initial.Content, initial.SourceHash, initial.SceneRevision, 0)
	if err != nil || !changedBytes || formatted.SourceHash == initial.SourceHash || *formatted.ContentHash != *initial.ContentHash {
		t.Fatalf("format-only save: %+v %t %v", formatted, changedBytes, err)
	}
	var formatOnly spec.SlideSpec
	if err := json.Unmarshal([]byte(formatted.Content), &formatOnly); err != nil || formatOnly.UpdatedAt != 1 {
		t.Fatalf("format-only changed updated_at: %+v %v", formatOnly, err)
	}
	changed := strings.Replace(initial.Content, `"key_message":"原始"`, `"key_message":"新内容"`, 1)
	saved, didChange, err := svc.Save(ctx, project.ID, "sli_source", "spec", changed, formatted.SourceHash, formatted.SceneRevision, 0)
	if err != nil || !didChange || saved.SourceHash == formatted.SourceHash || saved.ContentHash == nil || *saved.ContentHash == *formatted.ContentHash {
		t.Fatalf("save: %+v %t %v", saved, didChange, err)
	}
	if _, _, err = svc.Save(ctx, project.ID, "sli_source", "spec", changed, initial.SourceHash, initial.SceneRevision, 0); sourceCode(err) != "CONTENT_CONFLICT" {
		t.Fatalf("stale CAS: %v", err)
	}
	before, err := svc.History.State(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, didChange, err = svc.Save(ctx, project.ID, "sli_source", "spec", saved.Content, saved.SourceHash, saved.SceneRevision, 0)
	if err != nil || didChange {
		t.Fatalf("no-op: %t %v", didChange, err)
	}
	after, err := svc.History.State(project.ID)
	if err != nil || after.Revision != before.Revision {
		t.Fatalf("no-op history: %+v %+v %v", before, after, err)
	}
	after.Latest = "future-checkpoint"
	if err := svc.History.Save(project.ID, after); err != nil {
		t.Fatal(err)
	}
	if _, changed, err := svc.Save(ctx, project.ID, "sli_source", "spec", saved.Content, saved.SourceHash, saved.SceneRevision, 0); err != nil || changed {
		t.Fatalf("no-op with future must not require confirmation: %t %v", changed, err)
	}
	withFuture := strings.Replace(saved.Content, "新内容", "再次修改", 1)
	if _, _, err := svc.Save(ctx, project.ID, "sli_source", "spec", withFuture, saved.SourceHash, saved.SceneRevision, 0); sourceCode(err) != "HISTORY_CONFIRM_REQUIRED" {
		t.Fatalf("changed bytes must require confirmation: %v", err)
	}
	var parsed spec.SlideSpec
	if err := json.Unmarshal([]byte(saved.Content), &parsed); err != nil || parsed.KeyMessage != "新内容" {
		t.Fatalf("saved content: %+v %v", parsed, err)
	}
}

func TestSlideSourceIsReadOnlyDuringActiveRun(t *testing.T) {
	svc, project := sourceFixture(t)
	ctx := context.Background()
	if err := svc.History.Store.CreateThread(ctx, model.Thread{ID: "thr_source", ProjectID: project.ID, HistoryPath: "threads/thr_source.jsonl", Status: "active", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := svc.History.Store.CreateRun(ctx, model.Run{
		ID: "run_source", ThreadID: "thr_source", ProjectID: project.ID, Status: model.RunRunning,
		Command: model.RunCommand{Scope: model.NewRunScope(model.ScopeAllPages), Mode: model.ModeExecute, Instruction: "build"},
	}); err != nil {
		t.Fatal(err)
	}
	document, err := svc.Read(ctx, project.ID, "sli_source", "html")
	if err != nil || document.Writable || document.ReadonlyReason == nil || *document.ReadonlyReason != "RUN_ACTIVE" {
		t.Fatalf("active read: %+v %v", document, err)
	}
	if _, _, err := svc.Save(ctx, project.ID, "sli_source", "html", document.Content, document.SourceHash, document.SceneRevision, 0); sourceCode(err) != "RUN_ACTIVE" {
		t.Fatalf("active save: %v", err)
	}
}

func TestSlideSourceHTMLRejectsCommentStageAndNeverCreatesMissingFile(t *testing.T) {
	svc, project := sourceFixture(t)
	ctx := context.Background()
	initial, err := svc.Read(ctx, project.ID, "sli_source", "html")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = svc.Save(ctx, project.ID, "sli_source", "html", `<!-- slide-stage --><html><body></body></html>`, initial.SourceHash, initial.SceneRevision, 0); sourceCode(err) != "SOURCE_VALIDATION_FAILED" {
		t.Fatalf("comment accepted: %v", err)
	}
	path := filepath.Join(project.WorkDir, model.SlideHTMLPath("sli_source"))
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, _, err = svc.Save(ctx, project.ID, "sli_source", "html", initial.Content, initial.SourceHash, initial.SceneRevision, 0); sourceCode(err) != "SOURCE_NOT_FOUND" {
		t.Fatalf("missing file recreated: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file appeared: %v", err)
	}
}
