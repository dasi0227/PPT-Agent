package sqlite

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestRuntimeAuthorityMigrationCreatesTablesAndIndexes(t *testing.T) {
	s := newTestStore(t)
	for _, name := range []string{"idempotency_records", "steering_inbox"} {
		var count int64
		if err := s.db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("missing table %s", name)
		}
	}
	for _, name := range []string{"idx_runs_thread_client_request", "idx_versions_run_target"} {
		var count int64
		if err := s.db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", name).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("missing unique index %s", name)
		}
	}
}

func TestRunModelSelectionSnapshotRoundTripsWithoutKey(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateProject(ctx, model.Project{
		ID: "model-project", Title: "project", WorkDir: t.TempDir(),
		Status: "draft", CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateThread(ctx, model.Thread{
		ID: "model-thread", ProjectID: "model-project", HistoryPath: "thread.jsonl",
		Status: "active", CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	runModel := model.Run{
		ID: "model-run", ThreadID: "model-thread", ProjectID: "model-project",
		Model: model.ModelSelection{
			ProfileName: "Kimi Stable", Provider: "kimi",
			Model: "kimi-k3", URL: "https://gateway.example/v1",
		},
		Command: model.RunCommand{
			Scope: model.NewRunScope(model.ScopeObjectSpec, model.ScopeAllPages),
			Mode:  model.ModeChat, Instruction: "inspect",
		},
		Status: model.RunPending, CreatedAt: 1, UpdatedAt: 1,
	}
	if err := s.CreateRun(ctx, runModel); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetRun(ctx, runModel.ID)
	if err != nil || got.Model != runModel.Model {
		t.Fatalf("model snapshot changed: got=%+v err=%v", got.Model, err)
	}
	columns := tableColumnsForStoreTest(t, s, "runs")
	for _, column := range []string{"model_profile_name", "model_provider", "model_name", "model_url"} {
		if !columns[column] {
			t.Fatalf("runs table missing %s", column)
		}
	}
	if columns["model_key"] || columns["key"] || columns["api_key"] {
		t.Fatalf("runs table must never store provider keys: %v", columns)
	}
}

func tableColumnsForStoreTest(t *testing.T, s *Store, table string) map[string]bool {
	t.Helper()
	var rows []struct {
		Name string `gorm:"column:name"`
	}
	if err := s.db.Raw("PRAGMA table_info(" + table + ")").Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, row := range rows {
		out[row.Name] = true
	}
	return out
}

func TestAcquireIdempotencyConcurrentDuplicateHasOneAuthority(t *testing.T) {
	s := newTestStore(t)
	const workers = 24
	var created int32
	var mismatches int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			record, won, err := s.AcquireIdempotency(context.Background(), model.IdempotencyRecord{
				Scope: "tool_call", OwnerID: "run-1", Key: "call-1", RequestHash: "hash-a",
			})
			if err != nil || record.RequestHash != "hash-a" {
				atomic.AddInt32(&mismatches, 1)
			}
			if won {
				atomic.AddInt32(&created, 1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if mismatches != 0 || created != 1 {
		t.Fatalf("created=%d mismatches=%d", created, mismatches)
	}
	conflict, won, err := s.AcquireIdempotency(context.Background(), model.IdempotencyRecord{
		Scope: "tool_call", OwnerID: "run-1", Key: "call-1", RequestHash: "hash-b",
	})
	if err != nil || won || conflict.RequestHash != "hash-a" {
		t.Fatalf("hash conflict must return first authority: record=%+v won=%v err=%v", conflict, won, err)
	}
}

func TestSteeringInboxIsIdempotentAndOrdered(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateProject(ctx, model.Project{
		ID: "project-1", Title: "project", WorkDir: t.TempDir(), Status: "draft", CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateThread(ctx, model.Thread{
		ID: "thread-1", ProjectID: "project-1", HistoryPath: "threads/thread-1.jsonl",
		Status: "active", CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRun(ctx, model.Run{
		ID: "run-1", ThreadID: "thread-1", ProjectID: "project-1",
		Command: model.RunCommand{
			Scope: model.NewRunScope(model.ScopeObjectSpec, model.ScopeAllPages),
			Mode:  model.ModeExecute, Instruction: "test",
		},
		Status: model.RunRunning, CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	first, created, err := s.CreateSteering(ctx, model.SteeringMessage{
		RunID: "run-1", ThreadID: "thread-1", ClientMessageID: "msg-1",
		RequestHash: "hash-1", Content: "first", Status: model.SteeringAccepted, AcceptedAt: 10,
	})
	if err != nil || !created || first.Content != "first" {
		t.Fatalf("first create: %+v %v %v", first, created, err)
	}
	replay, created, err := s.CreateSteering(ctx, model.SteeringMessage{
		RunID: "run-1", ThreadID: "thread-1", ClientMessageID: "msg-1",
		RequestHash: "other-hash", Content: "changed", Status: model.SteeringAccepted, AcceptedAt: 11,
	})
	if err != nil || created || replay.RequestHash != "hash-1" || replay.Content != "first" {
		t.Fatalf("replay must preserve first authority: %+v %v %v", replay, created, err)
	}
	_, _, err = s.CreateSteering(ctx, model.SteeringMessage{
		RunID: "run-1", ThreadID: "thread-1", ClientMessageID: "msg-2",
		RequestHash: "hash-2", Content: "second", Status: model.SteeringAccepted, AcceptedAt: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	pending, err := s.ListPendingSteering(ctx, "run-1")
	if err != nil || len(pending) != 2 || pending[0].ClientMessageID != "msg-1" || pending[1].ClientMessageID != "msg-2" {
		t.Fatalf("pending order=%+v err=%v", pending, err)
	}
	if err := s.MarkSteering(ctx, "run-1", []string{"msg-1"}, model.SteeringInjected, 30, ""); err != nil {
		t.Fatal(err)
	}
	pending, _ = s.ListPendingSteering(ctx, "run-1")
	if len(pending) != 1 || pending[0].ClientMessageID != "msg-2" {
		t.Fatalf("injected message remained pending: %+v", pending)
	}
}
