package fileopen

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type memoryStore struct{ value Settings }

func (m *memoryStore) ReadFileSettings(context.Context) (Settings, error) { return m.value, nil }
func (m *memoryStore) WriteFileSettings(_ context.Context, value Settings) error {
	if value.Revision != m.value.Revision {
		return ErrConflict
	}
	value.Revision++
	m.value = value
	return nil
}
func fakeApplication(t *testing.T) string {
	t.Helper()
	app := filepath.Join(t.TempDir(), "My Editor.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte("plist"), 0600); err != nil {
		t.Fatal(err)
	}
	path, err := filepath.EvalSymlinks(app)
	if err != nil {
		t.Fatal(err)
	}
	return path
}
func TestOpenUsesLatestSettingAndLiteralArguments(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "slide $(touch bad); 中文.html")
	if err := os.WriteFile(file, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	resolved, _ := filepath.EvalSymlinks(file)
	app := fakeApplication(t)
	store := &memoryStore{}
	svc := NewService(store, root)
	svc.platform = "darwin"
	var executable string
	var actual []string
	calls := 0
	svc.run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		executable = name
		actual = args
		calls++
		return nil, nil
	}
	cases := []struct {
		mode string
		args []string
	}{
		{"system", []string{resolved}}, {"vscode", []string{"-b", "com.microsoft.VSCode", resolved}},
		{"textedit", []string{"-e", resolved}}, {"finder", []string{"-R", resolved}}, {"custom", []string{"-a", app, resolved}},
	}
	for _, tc := range cases {
		store.value = Settings{OpenWith: tc.mode, CustomAppPath: app}
		if err := svc.Open(context.Background(), file); err != nil {
			t.Fatal(err)
		}
		if executable != "/usr/bin/open" || !reflect.DeepEqual(actual, tc.args) {
			t.Fatalf("%s: %s %v", tc.mode, executable, actual)
		}
	}
	before := calls
	svc.run = func(context.Context, string, ...string) ([]byte, error) {
		calls++
		return nil, errors.New("app removed")
	}
	if err := svc.Open(context.Background(), file); err == nil || calls != before+1 {
		t.Fatalf("failed launch retried/fell back: %v calls=%d", err, calls)
	}
}
func TestOpenRejectsMissingFilesAndEscapingSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.html")
	if err := os.WriteFile(outside, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.html")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	svc := NewService(&memoryStore{value: Settings{OpenWith: "system"}}, root)
	svc.platform = "darwin"
	svc.run = func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("unexpected native invocation")
		return nil, nil
	}
	for _, path := range []string{outside, link, root, "relative.html", "https://example.com"} {
		if err := svc.Open(context.Background(), path); !errors.Is(err, ErrPath) {
			t.Errorf("%s: %v", path, err)
		}
	}
	if err := svc.Open(context.Background(), filepath.Join(root, "missing.html")); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}
func TestPickerCancellationAndCustomValidation(t *testing.T) {
	store := &memoryStore{value: Settings{OpenWith: "system"}}
	svc := NewService(store, t.TempDir())
	svc.platform = "darwin"
	svc.run = func(context.Context, string, ...string) ([]byte, error) { return []byte("\n"), nil }
	app, err := svc.PickApplication(context.Background())
	if err != nil || app != nil || store.value.OpenWith != "system" {
		t.Fatalf("cancel: %v %v", app, err)
	}
	path := fakeApplication(t)
	svc.run = func(context.Context, string, ...string) ([]byte, error) { return []byte(path + "/\n"), nil }
	app, err = svc.PickApplication(context.Background())
	if err != nil || app.Name != "My Editor" || app.Path != path {
		t.Fatalf("selection: %v %v", app, err)
	}
	saved, err := svc.Save(context.Background(), Settings{OpenWith: "custom", CustomAppPath: app.Path, CustomApps: []Application{*app}})
	if err != nil || saved.Revision != 1 || saved.CustomAppName != "My Editor" {
		t.Fatalf("save: %+v %v", saved, err)
	}
	if _, err = svc.Save(context.Background(), Settings{OpenWith: "finder"}); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	for _, edit := range []Settings{{OpenWith: "unknown"}, {OpenWith: "system", CustomAppPath: path}, {OpenWith: "custom", CustomAppPath: "/missing.app"}} {
		if _, err := svc.Save(context.Background(), edit); err == nil {
			t.Fatalf("accepted %+v", edit)
		}
	}
	if store.value.Revision != 1 {
		t.Fatal("failed save modified preference")
	}
}

func TestApplicationListDeduplicatesAndDeletesSelectedMissingApp(t *testing.T) {
	ctx := context.Background()
	first, second := fakeApplication(t), fakeApplication(t)
	store := &memoryStore{value: Settings{OpenWith: "system"}}
	svc := NewService(store, t.TempDir())
	saved, err := svc.Save(ctx, Settings{OpenWith: "system", CustomApps: []Application{{Path: first}, {Path: second}, {Path: first}}})
	if err != nil || len(saved.CustomApps) != 2 || saved.OpenWith != "system" {
		t.Fatalf("add: %+v %v", saved, err)
	}
	edit := saved.Settings
	edit.OpenWith, edit.CustomAppPath = "custom", first
	selected, err := svc.Save(ctx, edit)
	if err != nil {
		t.Fatal(err)
	}
	// An uninstalled registered app must not block editing or removing entries.
	if err := os.RemoveAll(first); err != nil {
		t.Fatal(err)
	}
	edit = selected.Settings
	edit.CustomApps = []Application{selected.CustomApps[0]}
	retained, err := svc.Save(ctx, edit)
	if err != nil || retained.CustomAppPath != first {
		t.Fatalf("delete other app: %+v %v", retained, err)
	}
	edit = retained.Settings
	edit.CustomApps = []Application{}
	deleted, err := svc.Save(ctx, edit)
	if err != nil || deleted.OpenWith != "system" || deleted.CustomAppPath != "" || len(deleted.CustomApps) != 0 {
		t.Fatalf("delete selected: %+v %v", deleted, err)
	}
	edit = deleted.Settings
	edit.OpenWith, edit.CustomAppPath = "custom", second
	if _, err := svc.Save(ctx, edit); !errors.Is(err, ErrInvalid) {
		t.Fatalf("selected unregistered app: %v", err)
	}
}
