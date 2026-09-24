package sqlite

import (
	"context"
	"errors"
	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/shortcuts"
	"go.uber.org/zap"
	"path/filepath"
	"testing"
)

func TestShortcutSettingsPersistAndRejectConflicts(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{DBPath: filepath.Join(t.TempDir(), "settings.db")}
	db, closeDB, err := Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	svc := shortcuts.NewService(s)
	initial, err := svc.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stale, err := svc.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	initial.Bindings["deck.theme"] = shortcuts.Binding{Code: "KeyB", Primary: true}
	initial.Bindings["trigger.command"] = shortcuts.Binding{Trigger: "!"}
	saved, err := svc.Save(ctx, initial)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Save(ctx, stale); !errors.Is(err, shortcuts.ErrConflict) {
		t.Fatalf("stale save accepted: %v", err)
	}
	saved.Bindings["deck.next"] = saved.Bindings["deck.previous"]
	if _, err = svc.Save(ctx, saved); err == nil {
		t.Fatal("duplicate shortcut accepted")
	}
	current, _ := svc.Get(ctx)
	current.Bindings["trigger.page"] = current.Bindings["trigger.command"]
	if _, err = svc.Save(ctx, current); err == nil {
		t.Fatal("duplicate trigger accepted")
	}
	closeDB()
	db, closeDB, err = Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB()
	s, err = NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	restored, err := shortcuts.NewService(s).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Revision != 1 || restored.Bindings["deck.theme"].Code != "KeyB" || restored.Bindings["trigger.command"].Trigger != "!" || restored.Bindings["deck.next"].Code != "ArrowRight" {
		t.Fatalf("persistence or failed-save atomicity: %+v", restored)
	}
	restored.Bindings = shortcuts.Defaults()
	if _, err = shortcuts.NewService(s).Save(ctx, restored); err != nil {
		t.Fatal(err)
	}
	_, overrides, err := s.ReadShortcutSettings(ctx)
	if err != nil || len(overrides) != 0 {
		t.Fatalf("restore defaults retained overrides: %v %v", overrides, err)
	}
}
