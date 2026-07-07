package service_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

// TestHistorySkipsCorruptLinesAndSortsBySeq 断言 History：
//   - json 解析失败的行跳过（不整体 500）
//   - 输出按 seq 升序排序（与磁盘 append 顺序无关）
func TestHistorySkipsCorruptLinesAndSortsBySeq(t *testing.T) {
	ctx := context.Background()
	work := t.TempDir()
	workDir := filepath.Join(work, "p1")
	if err := os.MkdirAll(filepath.Join(workDir, "threads"), 0o755); err != nil {
		t.Fatal(err)
	}

	lines := []string{
		`{"seq":2,"ts":200,"run_id":"r1","turn":"agent","type":"markdown","data":{"text":"b"}}`,
		`{corrupt`,
		`{"seq":1,"ts":100,"run_id":"r1","turn":"user","type":"user_turn","data":{"text":"a"}}`,
	}
	if err := os.WriteFile(filepath.Join(workDir, "threads/t1.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{DBPath: filepath.Join(work, "t.db"), WorkRoot: work}
	db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	if err := st.CreateProject(ctx, model.Project{ID: "p1", WorkDir: workDir, Title: "t", Theme: "d", Status: "ready", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(ctx, model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl", Status: "active", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}

	svc := service.NewThreadService(st)
	out, err := svc.History(ctx, "t1")
	if err != nil {
		t.Fatalf("history err: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 valid entries (corrupt skipped), got %d: %+v", len(out), out)
	}
	if getFloat(out[0], "seq") != 1 || getFloat(out[1], "seq") != 2 {
		t.Fatalf("expected sorted by seq asc, got seq[0]=%v seq[1]=%v", out[0]["seq"], out[1]["seq"])
	}
}

// 空文件应返回空数组而非 error。
func TestHistoryEmptyFileReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	work := t.TempDir()
	workDir := filepath.Join(work, "p1")
	if err := os.MkdirAll(filepath.Join(workDir, "threads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "threads/t1.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{DBPath: filepath.Join(work, "t.db"), WorkRoot: work}
	db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	_ = st.CreateProject(ctx, model.Project{ID: "p1", WorkDir: workDir, Title: "t", Theme: "d", Status: "ready", CreatedAt: now, UpdatedAt: now})
	_ = st.CreateThread(ctx, model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl", Status: "active", CreatedAt: now, UpdatedAt: now})

	out, err := service.NewThreadService(st).History(ctx, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || len(out) != 0 {
		t.Fatalf("expected empty slice, got %+v", out)
	}
}

func getFloat(m map[string]any, k string) float64 {
	if v, ok := m[k].(float64); ok {
		return v
	}
	return 0
}
