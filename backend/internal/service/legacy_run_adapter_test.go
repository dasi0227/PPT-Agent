package service_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
)

func adapterStore(t *testing.T) *sqlitestore.Store {
	t.Helper()
	cfg := &config.Config{DBPath: filepath.Join(t.TempDir(), "adapter.db")}
	db, cleanup, err := sqlitestore.Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	store, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	if err := store.CreateProject(context.Background(), model.Project{ID: "p", Title: "P", WorkDir: t.TempDir(), Theme: "default", Status: "draft", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertSlide(context.Background(), model.Slide{ID: "stable", ProjectID: "p", Idx: 0, Order: 0, Layout: "content", JSONPath: model.SlideJSONPath("stable"), HTMLPath: model.SlideHTMLPath("stable")}); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestLegacyRunAdapterMappings(t *testing.T) {
	adapter := service.NewLegacyRunAdapter(adapterStore(t))
	index := 0
	cases := []struct {
		req      service.LegacyRunRequest
		artifact model.Artifact
		level    model.TargetLevel
		intent   model.InteractionIntent
		clarify  model.ClarificationPolicy
	}{
		{service.LegacyRunRequest{Kind: model.KindOutline, Scope: model.ScopeOverview, Mode: model.ModeNormal, Instruction: "x"}, model.ArtifactBlueprint, model.TargetDeck, model.IntentApply, model.ClarifyWhenBlocked},
		{service.LegacyRunRequest{Kind: model.KindOutline, Scope: model.ScopePage, PageIndex: &index, Mode: model.ModeNormal, Instruction: "x"}, model.ArtifactBlueprint, model.TargetSlide, model.IntentApply, model.ClarifyWhenBlocked},
		{service.LegacyRunRequest{Kind: model.KindGenerate, Scope: model.ScopeOverview, Mode: model.ModeNormal, Instruction: "x"}, model.ArtifactPresentation, model.TargetDeck, model.IntentApply, model.ClarifyWhenBlocked},
		{service.LegacyRunRequest{Kind: model.KindEdit, Scope: model.ScopePage, PageIndex: &index, Mode: model.ModeTalk, Instruction: "x"}, model.ArtifactPresentation, model.TargetSlide, model.IntentConsult, model.ClarifyWhenBlocked},
		{service.LegacyRunRequest{Kind: model.KindEdit, Scope: model.ScopePage, PageIndex: &index, Mode: model.ModeAsk, Instruction: "x"}, model.ArtifactPresentation, model.TargetSlide, model.IntentApply, model.ClarifyBeforeApply},
	}
	for _, tc := range cases {
		spec, err := adapter.Adapt(context.Background(), "p", tc.req)
		if err != nil {
			t.Fatalf("adapt: %v", err)
		}
		if spec.Target.Artifact != tc.artifact || spec.Target.Level != tc.level || spec.Interaction.Intent != tc.intent || spec.Interaction.Clarification != tc.clarify {
			t.Errorf("unexpected mapping: %+v", spec)
		}
		if spec.Target.Level == model.TargetSlide && spec.Target.SlideID != "stable" {
			t.Errorf("stable id not resolved: %+v", spec.Target)
		}
	}
	if adapter.DeprecationCount() != uint64(len(cases)) {
		t.Fatalf("deprecation signal count=%d", adapter.DeprecationCount())
	}
}

func TestLegacyRepoRejected(t *testing.T) {
	adapter := service.NewLegacyRunAdapter(adapterStore(t))
	_, err := adapter.Adapt(context.Background(), "p", service.LegacyRunRequest{Kind: model.KindEdit, Scope: model.ScopeRepo, Instruction: "write"})
	if err == nil {
		t.Fatal("repo scope must not enter artifact runner")
	}
}
