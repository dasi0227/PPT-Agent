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

func TestPromptServiceConcurrentNameConflict(t *testing.T) {
	svc := newPromptServiceForTest(t)
	params := PromptWriteParams{Name: "并发摘要 / Concurrent Summary", Desc: "并发测试", Value: "内容"}
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
		case errors.Is(err, ErrPromptNameConflict):
			conflicts++
		default:
			t.Fatalf("unexpected create error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("want one success and one conflict, got successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestPromptServiceSeedsDefaultsIdempotently(t *testing.T) {
	svc := newPromptServiceForTest(t)
	ctx := context.Background()

	created, err := svc.SeedDefaults(ctx)
	if err != nil {
		t.Fatalf("seed defaults: %v", err)
	}
	if created != len(defaultPromptSeeds) {
		t.Fatalf("created=%d want=%d", created, len(defaultPromptSeeds))
	}
	created, err = svc.SeedDefaults(ctx)
	if err != nil {
		t.Fatalf("seed defaults again: %v", err)
	}
	if created != 0 {
		t.Fatalf("second seed created %d prompts", created)
	}

	prompts, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("list seeded prompts: %v", err)
	}
	if len(prompts) != len(defaultPromptSeeds) {
		t.Fatalf("seeded prompts=%d want=%d", len(prompts), len(defaultPromptSeeds))
	}
	covered := map[model.PromptTag]bool{}
	for _, prompt := range prompts {
		for _, tag := range prompt.Tags {
			covered[tag] = true
		}
	}
	for _, tag := range []model.PromptTag{
		model.PromptTagDeliverable, model.PromptTagReview,
	} {
		if !covered[tag] {
			t.Errorf("default prompts do not cover tag %q", tag)
		}
	}
}

func TestPromptServiceCRUDAndNormalization(t *testing.T) {
	svc := newPromptServiceForTest(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, PromptWriteParams{
		Name: " 高管摘要 / Executive Summary ", Desc: " 摘要说明 ", Value: " 生成高管摘要。 ",
		Tags: []model.PromptTag{model.PromptTagDeliverable, model.PromptTagReview},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Name != "高管摘要 / Executive Summary" || created.Desc != "摘要说明" || created.Value != "生成高管摘要。" || created.CreatedAt != 10 {
		t.Fatalf("unexpected create result: %+v", created)
	}
	if _, err := svc.Create(ctx, PromptWriteParams{
		Name: "高管摘要 / EXECUTIVE SUMMARY", Desc: "另一条说明", Value: "冲突",
	}); !errors.Is(err, ErrPromptNameConflict) {
		t.Fatalf("want name conflict, got %v", err)
	}

	svc.clock = func() int64 { return 20 }
	updated, err := svc.Update(ctx, created.ID, PromptWriteParams{
		Name: "摘要改写 / Summary Rewrite", Desc: "更新后的说明", Value: "更新后的内容",
		Tags: []model.PromptTag{model.PromptTagDeliverable},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.CreatedAt != 10 || updated.UpdatedAt != 20 {
		t.Fatalf("timestamps changed incorrectly: %+v", updated)
	}
	disabled, err := svc.SetDisabled(ctx, created.ID, true)
	if err != nil || !disabled.Disabled {
		t.Fatalf("disable prompt: %+v, %v", disabled, err)
	}
	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "摘要改写 / Summary Rewrite" {
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
		{Name: "", Desc: "desc", Value: "value"},
		{Name: "invalid\nname", Desc: "desc", Value: "value"},
		{Name: "name", Desc: "", Value: "value"},
		{Name: "name", Desc: "desc", Value: ""},
		{Name: "name", Desc: "desc", Value: "value", Tags: []model.PromptTag{"unknown"}},
		{Name: "name", Desc: "desc", Value: "value", Tags: []model.PromptTag{model.PromptTagIdentity, model.PromptTagIdentity}},
		{Name: "name", Desc: "desc", Value: "value", Tags: []model.PromptTag{model.PromptTagIdentity, model.PromptTagConstraint, model.PromptTagReview}},
	}
	for index, params := range tests {
		if _, err := svc.Create(context.Background(), params); !errors.Is(err, ErrPromptInvalid) {
			t.Errorf("case %d: want invalid, got %v", index, err)
		}
	}
}
