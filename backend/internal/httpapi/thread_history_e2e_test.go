package httpapi_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/httpapi"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

// setupServerWithHistory 起 gin server 并挂上真实 FSHistoryWriter，seed project+thread；
// project.WorkDir 使用 t.TempDir 子目录，确保 append 可写入。
func setupServerWithHistory(t *testing.T, runner run.Runner) (*httptest.Server, string, string) {
	t.Helper()
	root := t.TempDir()
	workDir := filepath.Join(root, "p1")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{DBPath: filepath.Join(root, "e2e.db"), WorkRoot: root}
	db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	ctx := context.Background()
	now := time.Now().Unix()
	if err := st.CreateProject(ctx, model.Project{ID: "p1", Title: "t", WorkDir: workDir, Theme: "default", Status: "ready", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateThread(ctx, model.Thread{ID: "th1", ProjectID: "p1", HistoryPath: "threads/th1.jsonl", Status: "active", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}

	hw := run.NewFSHistoryWriter(st)
	engine := run.NewEngine(st, run.NewLockManager(), hw, zap.NewNop())
	runSvc := service.NewRunServiceWithFactory(st, engine, func(r model.Run, p model.CreateRunParams, proj model.Project) run.Runner {
		return runner
	})
	router := httpapi.NewRouter(
		cfg, zap.NewNop(),
		httpapi.NewHealthHandler(service.NewHealthService(st)),
		httpapi.NewRunHandler(runSvc),
		httpapi.NewProjectHandler(service.NewProjectService(st, service.WorkRoot(root)), service.NewSlideService(st)),
		httpapi.NewThreadHandler(service.NewThreadService(st)),
		httpapi.NewSlideHandler(service.NewSlideService(st)),
		httpapi.NewAssetHandler(service.NewAssetService(st, root)),
	)
	srv := httptest.NewServer(router.Engine())
	t.Cleanup(srv.Close)
	return srv, "th1", workDir
}

// scriptedTurn 是 e2e 用的最小 runner：发 run.started(user_input=xxx) + info + finish。
type scriptedTurn struct {
	runID string
	instr string
}

func (r scriptedTurn) Run(ctx context.Context, em harness.Emitter, _ harness.Checkpointer, _ run.Prompter) harness.Outcome {
	em.Emit(model.EventRunStarted, harness.RunStartedPayload{
		RunID:       r.runID,
		Target:      &model.RunTarget{Artifact: model.ArtifactBlueprint, Level: model.TargetDeck},
		Interaction: &model.RunInteraction{Intent: model.IntentConsult, Clarification: model.ClarifyNever},
		UserInput:   r.instr,
	})
	em.Emit(model.EventInfo, harness.InfoPayload{Text: "acknowledged"})
	return harness.Outcome{Status: harness.OutcomeFinished, Summary: "ok"}
}

// TestThreadHistoryE2E_UserTurnAndFinalResultLanded：
//
//	一次 run 结束后，thread 的 history.jsonl 至少应含 user_turn + final_result；
//	GET /threads/:id/history 返回同数据（按 seq 升序）。
func TestThreadHistoryE2E_UserTurnAndFinalResultLanded(t *testing.T) {
	instr := "帮我写一个开场页"
	srv, threadID, workDir := setupServerWithHistory(t, scriptedTurn{runID: "auto", instr: instr})

	// 创建 run 并等待 SSE done。
	runID := createHistoryRun(t, srv, threadID, instr)
	waitDoneOrTimeout(t, srv, runID)

	// 磁盘断言。
	path := filepath.Join(workDir, "threads", threadID+".jsonl")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read history.jsonl: %v", err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		t.Fatalf("history.jsonl is empty at %s", path)
	}
	if !strings.Contains(string(raw), `"type":"user_turn"`) {
		t.Fatalf("expected user_turn in history, got: %s", string(raw))
	}
	if !strings.Contains(string(raw), `"type":"final_result"`) {
		t.Fatalf("expected final_result in history, got: %s", string(raw))
	}
	// info→markdown 也应存在。
	if !strings.Contains(string(raw), `"type":"markdown"`) {
		t.Fatalf("expected markdown (info) in history, got: %s", string(raw))
	}

	// API 端点断言。
	resp, err := http.Get(srv.URL + "/api/v1/threads/" + threadID + "/history")
	if err != nil {
		t.Fatalf("GET history: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET history status=%d body=%s", resp.StatusCode, string(body))
	}
	var out []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out) < 3 {
		t.Fatalf("expected >=3 entries via API, got %d: %+v", len(out), out)
	}
	if out[0]["type"] != "user_turn" {
		t.Fatalf("expected first entry user_turn, got %v", out[0]["type"])
	}
	if data, ok := out[0]["data"].(map[string]any); !ok || data["text"] != instr {
		t.Fatalf("expected user_turn text=%q, got %+v", instr, out[0])
	}
	if out[len(out)-1]["type"] != "final_result" {
		t.Fatalf("expected last entry final_result, got %v", out[len(out)-1]["type"])
	}
	// seq 严格递增。
	var lastSeq float64
	for i, entry := range out {
		s, _ := entry["seq"].(float64)
		if i > 0 && s <= lastSeq {
			t.Fatalf("expected seq strictly increasing, got %v after %v", s, lastSeq)
		}
		lastSeq = s
	}
}

func createHistoryRun(t *testing.T, srv *httptest.Server, threadID, instr string) string {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{
		"target":      map[string]any{"artifact": "blueprint", "level": "deck"},
		"interaction": map[string]any{"intent": "consult", "clarification": "never"},
		"instruction": instr,
	})
	resp, err := http.Post(srv.URL+"/api/v1/threads/"+threadID+"/runs", "application/json", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("create run status=%d body=%s", resp.StatusCode, string(b))
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out.ID == "" {
		t.Fatal("empty run id")
	}
	return out.ID
}

// waitDoneOrTimeout 读 SSE 到终态，或 3s 超时（避免测试挂死）。
func waitDoneOrTimeout(t *testing.T, srv *httptest.Server, runID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/runs/"+runID+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("sse: %v", err)
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "event:") {
			ev := strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			if ev == "done" || ev == "error" {
				return
			}
		}
	}
	t.Fatalf("SSE stream ended without done/error (sc.err=%v)", sc.Err())
}
