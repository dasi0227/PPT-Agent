package run

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type fakeLoc struct {
	proj model.Project
	thr  model.Thread
}

func (f *fakeLoc) GetThread(_ context.Context, id string) (model.Thread, error) {
	if f.thr.ID != id {
		return model.Thread{}, os.ErrNotExist
	}
	return f.thr, nil
}
func (f *fakeLoc) GetProject(_ context.Context, id string) (model.Project, error) {
	if f.proj.ID != id {
		return model.Project{}, os.ErrNotExist
	}
	return f.proj, nil
}

func TestFSHistoryWriter_AppendCreatesFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "threads"), 0o755); err != nil {
		t.Fatal(err)
	}
	loc := &fakeLoc{
		proj: model.Project{ID: "p1", WorkDir: dir},
		thr:  model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl"},
	}
	w := NewFSHistoryWriter(loc)
	err := w.Append(context.Background(), "t1", HistoryEntry{Seq: 1, TS: 100, RunID: "r1", Turn: "user", Type: "user_turn", Data: map[string]any{"text": "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "threads/t1.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(raw))
	var got HistoryEntry
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatal(err)
	}
	if got.Seq != 1 || got.Turn != "user" || got.Data["text"] != "hi" {
		t.Fatalf("unexpected entry: %+v", got)
	}
}

func TestFSHistoryWriter_AppendsMultipleLines(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "threads"), 0o755)
	loc := &fakeLoc{
		proj: model.Project{ID: "p1", WorkDir: dir},
		thr:  model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl"},
	}
	w := NewFSHistoryWriter(loc)
	for i := 0; i < 3; i++ {
		if err := w.Append(context.Background(), "t1", HistoryEntry{Seq: int64(i), Turn: "agent", Type: "info", Data: map[string]any{"text": "x"}}); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "threads/t1.jsonl"))
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %s", len(lines), string(raw))
	}
}

func TestFSHistoryWriter_ConcurrentSameThread(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "threads"), 0o755)
	loc := &fakeLoc{
		proj: model.Project{ID: "p1", WorkDir: dir},
		thr:  model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl"},
	}
	w := NewFSHistoryWriter(loc)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(seq int) {
			defer wg.Done()
			_ = w.Append(context.Background(), "t1", HistoryEntry{Seq: int64(seq), Turn: "agent", Type: "info", Data: map[string]any{"text": "x"}})
		}(i)
	}
	wg.Wait()
	raw, _ := os.ReadFile(filepath.Join(dir, "threads/t1.jsonl"))
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 50 {
		t.Fatalf("expected 50 lines, got %d", len(lines))
	}
	for _, l := range lines {
		var e HistoryEntry
		if err := json.Unmarshal([]byte(l), &e); err != nil {
			t.Fatalf("invalid json line: %s", l)
		}
	}
}

func TestFSHistoryWriter_UnknownThreadReturnsError(t *testing.T) {
	dir := t.TempDir()
	loc := &fakeLoc{
		proj: model.Project{ID: "p1", WorkDir: dir},
		thr:  model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl"},
	}
	w := NewFSHistoryWriter(loc)
	err := w.Append(context.Background(), "unknown", HistoryEntry{Seq: 1})
	if err == nil {
		t.Fatal("expected error for unknown thread")
	}
}
