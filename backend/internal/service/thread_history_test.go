package service_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
	"go.uber.org/zap"
)

func historyFixture(t *testing.T) (*sqlitestore.Store, *service.ThreadService, string) {
	t.Helper()
	root := t.TempDir()
	db, cleanup, err := sqlitestore.Open(&config.Config{WorkRoot: root, DBPath: filepath.Join(root, "test.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	s, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateProject(context.Background(), model.Project{ID: "p", Title: "project", Theme: "theme", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateThread(context.Background(), model.Thread{ID: "t", ProjectID: "p", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	return s, service.NewThreadService(s), filepath.Join(root, "projects", "p", "threads", "t", "thread.jsonl")
}
func TestHistoryUsesPublicJournalProjection(t *testing.T) {
	s, svc, _ := historyFixture(t)
	ctx := context.Background()
	if _, err := s.AppendThreadEvent(ctx, "t", threadjournal.Event{Type: "diagnostic.private", Payload: json.RawMessage(`{"secret":"hidden"}`)}); err != nil {
		t.Fatal(err)
	}
	command, _, err := s.AcceptCommand(ctx, "t", model.CommandRequest{RequestKey: "command", Kind: "polish", Input: json.RawMessage(`{"instruction":"input"}`)}, 0)
	if err != nil {
		t.Fatal(err)
	}
	command.Status = "completed"
	command.Result = json.RawMessage(`{"content":"result"}`)
	if err := s.SaveCommandExecution(ctx, command); err != nil {
		t.Fatal(err)
	}
	history, err := svc.History(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0]["type"] != "command.accepted" || history[1]["type"] != "command.completed" {
		t.Fatalf("public history: %+v", history)
	}
}
func TestHistoryRejectsCompleteCorruptLine(t *testing.T) {
	_, svc, path := historyFixture(t)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("{invalid}\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.History(context.Background(), "t"); err == nil {
		t.Fatal("silently skipped corrupt history")
	}
}
func TestNewThreadHasEmptyPublicHistory(t *testing.T) {
	_, svc, _ := historyFixture(t)
	history, err := svc.History(context.Background(), "t")
	if err != nil || history == nil || len(history) != 0 {
		t.Fatalf("history=%+v err=%v", history, err)
	}
}
