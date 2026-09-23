package service_test

import (
	"context"
	"encoding/json"
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

func TestHistoryRecoversDatabaseEventsAndCommandsWithoutJSONL(t *testing.T) {
	ctx := context.Background()
	work := t.TempDir()
	db, cleanup, err := sqlitestore.Open(&config.Config{DBPath: filepath.Join(work, "test.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	workDir := filepath.Join(work, "artifacts")
	if err = os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err = st.CreateProject(ctx, model.Project{ID: "p", WorkDir: workDir, Title: "Project", Status: "ready", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err = st.CreateThread(ctx, model.Thread{ID: "t", ProjectID: "p", HistoryPath: model.UserHistoryPath("t"), Status: "active", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err = db.Exec(`INSERT INTO runs(id,thread_id,project_id,scope_slide_ids_json,scope_source_json,scope_revision,mode,run_command_json,status,created_at,updated_at) VALUES ('r','t','p','[]','{}',1,'chat','{}','done',1,2)`).Error; err != nil {
		t.Fatal(err)
	}
	for _, event := range []model.Event{
		{RunID: "r", Seq: 1, Type: model.EventRunStarted, Payload: `{"user_input":"制作介绍","mode":"chat","scope":{}}`, CreatedAt: 1},
		{RunID: "r", Seq: 2, Type: model.EventCommandPermissionRequested, Payload: `{"interaction_id":"permission"}`, CreatedAt: 1},
		{RunID: "r", Seq: 3, Type: model.EventCommandPermissionAnswered, Payload: `{"interaction_id":"permission","decision":"allow"}`, CreatedAt: 1},
	} {
		if err = st.AppendEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	activity := model.CommandActivity{ID: "rename:1", AttemptID: "first", ThreadID: "t", ProjectID: "p", Kind: "rename", Method: "auto", Status: "completed", Phase: 2, Request: json.RawMessage("{}"), Result: json.RawMessage(`{"title":""}`), CreatedAt: 2000, UpdatedAt: 2000}
	if err = st.SaveCommandActivity(ctx, activity); err != nil {
		t.Fatal(err)
	}
	history, err := service.NewThreadService(st).History(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 4 {
		t.Fatalf("history missing persisted events: %+v", history)
	}
	for index, kind := range []string{"user_turn", "command.permission_requested", "command.permission_answered", "command_activity"} {
		if history[index]["type"] != kind {
			t.Fatalf("history order mismatch: %+v", history)
		}
	}
}

// TestHistorySkipsCorruptLinesAndSortsBySeq 断言 History：
//   - json 解析失败的行跳过（不整体 500）
//   - 输出按 seq 升序排序（与磁盘 append 顺序无关）
func TestHistorySkipsCorruptLinesAndSortsBySeq(t *testing.T) {
	ctx := context.Background()
	work := t.TempDir()
	workDir := filepath.Join(work, "projects", "p1", "artifacts")
	threadDir := filepath.Join(model.ProjectRoot(workDir), "threads", "t1")
	if err := os.MkdirAll(threadDir, 0o755); err != nil {
		t.Fatal(err)
	}

	lines := []string{
		`{"seq":2,"ts":200,"run_id":"r1","turn":"agent","type":"markdown","data":{"text":"b"}}`,
		`{corrupt`,
		`{"seq":1,"ts":100,"run_id":"r1","turn":"user","type":"user_turn","data":{"text":"a"}}`,
	}
	if err := os.WriteFile(filepath.Join(threadDir, "user.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
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
	if err := st.CreateThread(ctx, model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: model.UserHistoryPath("t1"), Status: "active", CreatedAt: now, UpdatedAt: now}); err != nil {
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
	workDir := filepath.Join(work, "projects", "p1", "artifacts")
	threadDir := filepath.Join(model.ProjectRoot(workDir), "threads", "t1")
	if err := os.MkdirAll(threadDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(threadDir, "user.jsonl"), []byte(""), 0o644); err != nil {
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
	_ = st.CreateThread(ctx, model.Thread{ID: "t1", ProjectID: "p1", HistoryPath: model.UserHistoryPath("t1"), Status: "active", CreatedAt: now, UpdatedAt: now})

	out, err := service.NewThreadService(st).History(ctx, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || len(out) != 0 {
		t.Fatalf("expected empty slice, got %+v", out)
	}
}

func TestHistoryMergesBriefingGroupWithoutWritingJSONL(t *testing.T) {
	ctx := context.Background()
	work := t.TempDir()
	workDir := filepath.Join(work, "projects", "p1", "artifacts")
	threadDir := filepath.Join(model.ProjectRoot(workDir), "threads", "t1")
	if err := os.MkdirAll(threadDir, 0o755); err != nil {
		t.Fatal(err)
	}
	historyPath := filepath.Join(threadDir, "user.jsonl")
	initial := strings.Join([]string{
		`{"seq":1,"ts":100,"run_id":"r1","turn":"user","type":"user_turn","data":{"text":"first"}}`,
		`{"seq":1,"ts":300,"run_id":"r2","turn":"user","type":"user_turn","data":{"text":"second"}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(historyPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	db, cleanup, err := sqlitestore.Open(&config.Config{DBPath: filepath.Join(work, "t.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateProject(ctx, model.Project{
		ID: "p1", WorkDir: workDir, Title: "t", Theme: "d",
		Status: "ready", CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(ctx, model.Thread{
		ID: "t1", ProjectID: "p1", HistoryPath: model.UserHistoryPath("t1"),
		Status: "active", CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	for versionNo, content := range []string{"v1", "v2"} {
		if err := st.AppendBriefingVersion(ctx, model.BriefingVersion{
			BriefingID: "b1", ThreadID: "t1", ProjectID: "p1",
			Kind: model.BriefingHandoff, VersionNo: versionNo + 1,
			Title: "交接任务", Content: content, CreatedAt: int64(199 + versionNo),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.CreateContextCompaction(ctx, model.ContextCompaction{
		ID: "cmp_1", ThreadID: "t1", ProjectID: "p1",
		Trigger: model.ContextCompactionManual, Title: "整理项目上下文", Content: "summary",
		BeforeTokens: 56000, AfterTokens: 30000, MaxTokens: 65536,
		Reclaimed: 26000, DurationMS: 3600, CreatedAt: 201,
	}); err != nil {
		t.Fatal(err)
	}
	out, err := service.NewThreadService(st).History(ctx, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 4 || out[1]["type"] != "briefing" || out[2]["type"] != "context_compaction" {
		t.Fatalf("sidecar events were not merged by timestamp: %+v", out)
	}
	compactionData, ok := out[2]["data"].(map[string]any)
	if !ok || compactionData["title"] != "整理项目上下文" || compactionData["content"] != "summary" {
		t.Fatalf("compaction title missing: %+v", out[2])
	}
	data, ok := out[1]["data"].(map[string]any)
	if !ok {
		t.Fatalf("briefing data missing: %+v", out[1])
	}
	versions, ok := data["versions"].([]model.BriefingVersion)
	if !ok || len(versions) != 2 || versions[1].Title != "交接任务" {
		t.Fatalf("briefing versions missing: %+v", data)
	}
	after, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != initial || strings.Contains(string(after), "briefing") {
		t.Fatalf("History wrote briefing into jsonl: %s", after)
	}
}

func getFloat(m map[string]any, k string) float64 {
	if v, ok := m[k].(float64); ok {
		return v
	}
	return 0
}
