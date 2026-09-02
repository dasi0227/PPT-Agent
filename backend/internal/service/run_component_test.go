package service

import (
	"context"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	runpkg "github.com/dasi0227/PPT-Agent/backend/internal/run"
	sqlitestore "github.com/dasi0227/PPT-Agent/backend/internal/store/sqlite"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type componentNoOpExecution struct{}

func (componentNoOpExecution) Run(
	context.Context,
	workflow.EventEmitter,
	runpkg.Checkpointer,
	runpkg.Prompter,
) workflow.StructuredOutcome {
	return workflow.StructuredOutcome{Status: workflow.StatusCompleted}
}

func TestCreateRunResolvesComponentNamesIntoCommandSnapshot(t *testing.T) {
	root := t.TempDir()
	db, cleanup, err := sqlitestore.Open(&config.Config{DBPath: filepath.Join(root, "run.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	store, err := sqlitestore.NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.CreateProject(ctx, model.Project{
		ID: "p1", Title: "Project", WorkDir: filepath.Join(root, "project"),
		Theme: "default", Status: "draft", CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateThread(ctx, model.Thread{
		ID: "t1", ProjectID: "p1", HistoryPath: "threads/t1.jsonl",
		Status: "active", CreatedAt: 1, UpdatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	writeRepositoryFile(
		t,
		filepath.Join(root, "assets/components/feature-card/index.html"),
		componentFile("能力卡片", "Feature card", "<section>complete html</section>"),
	)

	engine := runpkg.NewEngine(store, runpkg.NewLockManager(), nil, zap.NewNop())
	service := NewRunServiceWithExecutionFactory(
		store,
		engine,
		func(model.Run, model.CreateRunParams, model.Project) runpkg.Execution {
			return componentNoOpExecution{}
		},
	)
	service.components = NewComponentService(WorkRoot(root), store)

	created, err := service.CreateRun(ctx, "t1", model.CreateRunParams{
		ClientRequestID: "component-request",
		ComponentNames:  []string{"能力卡片"},
		Command: model.RunCommand{
			Scope: model.RunScope{Artifact: model.ArtifactPPT, Level: model.ScopeDeck},
			Mode:  model.ModeExecute, Instruction: "参考能力卡片",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Command.Components) != 1 ||
		created.Command.Components[0].ID != "feature-card" ||
		created.Command.Components[0].Name != "能力卡片" ||
		created.Command.Components[0].HTML == "" {
		t.Fatalf("component snapshot=%+v", created.Command.Components)
	}
}
