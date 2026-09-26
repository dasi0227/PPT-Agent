package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/fileopen"
	"go.uber.org/zap"
)

func TestFileSettingsPersistAndRejectConflicts(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cfg := &config.Config{WorkRoot: root, DBPath: filepath.Join(root, "settings.db")}
	db, closeDB, err := Open(cfg, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	svc := fileopen.NewService(s, root)
	initial, err := svc.Get(ctx)
	if err != nil || initial.OpenWith != "system" || initial.Revision != 0 {
		t.Fatalf("defaults: %+v %v", initial, err)
	}
	edit := initial.Settings
	edit.OpenWith = "finder"
	app := filepath.Join(root, "My Editor.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte("plist"), 0600); err != nil {
		t.Fatal(err)
	}
	app, err = filepath.EvalSymlinks(app)
	if err != nil {
		t.Fatal(err)
	}
	edit.CustomApps = []fileopen.Application{{Name: "My Editor", Path: app}}
	if _, err = svc.Save(ctx, edit); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Save(ctx, initial.Settings); !errors.Is(err, fileopen.ErrConflict) {
		t.Fatal(err)
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
	restored, err := s.ReadFileSettings(ctx)
	if err != nil || restored.OpenWith != "finder" || restored.Revision != 1 || restored.CustomAppPath != "" || len(restored.CustomApps) != 1 || restored.CustomApps[0].Path != app || restored.CustomApps[0].Name != "My Editor" {
		t.Fatalf("persistence: %+v %v", restored, err)
	}
}
