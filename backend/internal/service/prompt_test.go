package service

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"go.uber.org/zap"
)

func newPromptServiceForTest(t *testing.T) *PromptService {
	t.Helper()
	db, cleanup, err := sqlitestore.Open(&config.Config{DBPath: filepath.Join(t.TempDir(), "prompt.db")}, zap.NewNop())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(cleanup)
	st, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	svc := NewPromptService(st)
	var nextID atomic.Int64
	svc.newID = func() string {
		return "prm_test_" + strconv.FormatInt(nextID.Add(1), 10)
	}
	svc.clock = func() int64 { return 10 }
	return svc
}

func TestPromptServiceConcurrentKeyConflict(t *testing.T) {
	svc := newPromptServiceForTest(t)
	params := PromptWriteParams{KeyZH: "并发摘要", KeyEN: "concurrent-summary", Value: "内容"}
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, err := svc.Create(context.Background(), params)
			results <- err
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrPromptKeyConflict):
			conflicts++
		default:
			t.Fatalf("unexpected create error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("want one success and one conflict, got successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestPromptServiceCRUDAndNormalization(t *testing.T) {
	svc := newPromptServiceForTest(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, PromptWriteParams{
		KeyZH: " 高管摘要 ", KeyEN: "Executive-Summary", Value: " 生成高管摘要。 ",
		Tags: []model.PromptTag{model.PromptTagSummarize, model.PromptTagRewrite},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.KeyZH != "高管摘要" || created.Value != "生成高管摘要。" || created.CreatedAt != 10 {
		t.Fatalf("unexpected create result: %+v", created)
	}
	if _, err := svc.Create(ctx, PromptWriteParams{
		KeyZH: "另一个中文", KeyEN: "executive-summary", Value: "冲突",
	}); !errors.Is(err, ErrPromptKeyConflict) {
		t.Fatalf("want key conflict, got %v", err)
	}

	svc.clock = func() int64 { return 20 }
	updated, err := svc.Update(ctx, created.ID, PromptWriteParams{
		KeyZH: "摘要改写", KeyEN: "summary-rewrite", Value: "更新后的内容",
		Tags: []model.PromptTag{model.PromptTagRewrite},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.CreatedAt != 10 || updated.UpdatedAt != 20 {
		t.Fatalf("timestamps changed incorrectly: %+v", updated)
	}
	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 || list[0].KeyZH != "摘要改写" {
		t.Fatalf("unexpected list: %+v, %v", list, err)
	}
	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.Get(ctx, created.ID); !errors.Is(err, ErrPromptNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestPromptServiceValidation(t *testing.T) {
	svc := newPromptServiceForTest(t)
	tests := []PromptWriteParams{
		{KeyZH: "english", KeyEN: "valid", Value: "value"},
		{KeyZH: "中文 key", KeyEN: "valid", Value: "value"},
		{KeyZH: "中文", KeyEN: "bad key", Value: "value"},
		{KeyZH: "中文", KeyEN: "valid", Value: ""},
		{KeyZH: "中文", KeyEN: "valid", Value: "value", Tags: []model.PromptTag{"unknown"}},
		{KeyZH: "中文", KeyEN: "valid", Value: "value", Tags: []model.PromptTag{model.PromptTagData, model.PromptTagData}},
		{KeyZH: "中文", KeyEN: "valid", Value: "value", Tags: []model.PromptTag{model.PromptTagData, model.PromptTagVisual, model.PromptTagReview}},
	}
	for index, params := range tests {
		if _, err := svc.Create(context.Background(), params); !errors.Is(err, ErrPromptInvalid) {
			t.Errorf("case %d: want invalid, got %v", index, err)
		}
	}
}
