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

func settingsWithDefault(mode, path string) Settings {
	value := DefaultSettings()
	value.Default = Method{OpenWith: mode, CustomAppPath: path}
	return value
}

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
		store.value = settingsWithDefault(tc.mode, app)
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
	svc := NewService(&memoryStore{value: DefaultSettings()}, root)
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
	store := &memoryStore{value: DefaultSettings()}
	svc := NewService(store, t.TempDir())
	svc.platform = "darwin"
	svc.run = func(context.Context, string, ...string) ([]byte, error) { return []byte("\n"), nil }
	app, err := svc.PickApplication(context.Background())
	if err != nil || app != nil || store.value.Default.OpenWith != "system" {
		t.Fatalf("cancel: %v %v", app, err)
	}
	path := fakeApplication(t)
	svc.run = func(context.Context, string, ...string) ([]byte, error) { return []byte(path + "/\n"), nil }
	app, err = svc.PickApplication(context.Background())
	if err != nil || app.Name != "My Editor" || app.Path != path {
		t.Fatalf("selection: %v %v", app, err)
	}
	saved, err := svc.Save(context.Background(), Settings{Default: Method{OpenWith: "custom", CustomAppPath: app.Path}, JSON: Method{OpenWith: "inherit"}, HTML: Method{OpenWith: "inherit"}, CustomApps: []Application{*app}})
	if err != nil || saved.Revision != 1 || saved.Default.CustomAppPath != app.Path {
		t.Fatalf("save: %+v %v", saved, err)
	}
	if _, err = svc.Save(context.Background(), settingsWithDefault("finder", "")); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	for _, edit := range []Settings{settingsWithDefault("unknown", ""), settingsWithDefault("system", path), settingsWithDefault("custom", "/missing.app")} {
		edit.Revision = store.value.Revision
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
	store := &memoryStore{value: DefaultSettings()}
	svc := NewService(store, t.TempDir())
	saved, err := svc.Save(ctx, Settings{Default: Method{OpenWith: "system"}, JSON: Method{OpenWith: "inherit"}, HTML: Method{OpenWith: "inherit"}, CustomApps: []Application{{Path: first}, {Path: second}, {Path: first}}})
	if err != nil || len(saved.CustomApps) != 2 || saved.Default.OpenWith != "system" {
		t.Fatalf("add: %+v %v", saved, err)
	}
	edit := saved.Settings
	edit.Default.OpenWith, edit.Default.CustomAppPath = "custom", first
	edit.JSON = Method{OpenWith: "custom", CustomAppPath: first}
	edit.HTML = Method{OpenWith: "custom", CustomAppPath: second}
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
	if err != nil || retained.Default.CustomAppPath != first || retained.HTML.OpenWith != "inherit" {
		t.Fatalf("delete other app: %+v %v", retained, err)
	}
	edit = retained.Settings
	edit.CustomApps = []Application{}
	deleted, err := svc.Save(ctx, edit)
	if err != nil || deleted.Default.OpenWith != "system" || deleted.Default.CustomAppPath != "" || deleted.JSON.OpenWith != "inherit" || len(deleted.CustomApps) != 0 {
		t.Fatalf("delete selected: %+v %v", deleted, err)
	}
	edit = deleted.Settings
	edit.Default.OpenWith, edit.Default.CustomAppPath = "custom", second
	if _, err := svc.Save(ctx, edit); !errors.Is(err, ErrInvalid) {
		t.Fatalf("selected unregistered app: %v", err)
	}
}

func TestOpenRoutesFileTypesAndInheritedMethods(t *testing.T) {
	root := t.TempDir()
	settings := DefaultSettings()
	settings.Default = Method{OpenWith: "finder"}
	settings.JSON = Method{OpenWith: "textedit"}
	settings.HTML = Method{OpenWith: "vscode"}
	store := &memoryStore{value: settings}
	svc := NewService(store, root)
	svc.platform = "darwin"
	var actual []string
	svc.run = func(_ context.Context, _ string, args ...string) ([]byte, error) { actual = args; return nil, nil }
	for _, tc := range []struct {
		name   string
		prefix []string
	}{
		{".manifest.JSON", []string{"-e"}}, {"slide.html", []string{"-b", "com.microsoft.VSCode"}},
		{"slide.HTM", []string{"-b", "com.microsoft.VSCode"}}, {"SKILL.md", []string{"-R"}},
	} {
		path := filepath.Join(root, tc.name)
		if err := os.WriteFile(path, []byte("source"), 0600); err != nil {
			t.Fatal(err)
		}
		resolved, _ := filepath.EvalSymlinks(path)
		if err := svc.Open(context.Background(), path); err != nil {
			t.Fatal(err)
		}
		expected := append(append([]string{}, tc.prefix...), resolved)
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("%s: %v", tc.name, actual)
		}
	}
	store.value.HTML = Method{OpenWith: "inherit"}
	if err := svc.Open(context.Background(), filepath.Join(root, "slide.html")); err != nil {
		t.Fatal(err)
	}
	if actual[0] != "-R" {
		t.Fatalf("inherit: %v", actual)
	}
	edit := store.value
	edit.Default = Method{OpenWith: "inherit"}
	if _, err := svc.Save(context.Background(), edit); !errors.Is(err, ErrInvalid) {
		t.Fatalf("default inheritance accepted: %v", err)
	}
}

func TestPickedPresetCannotBecomeDeletableCustomApplication(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Visual Studio Code.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte("plist"), 0600); err != nil {
		t.Fatal(err)
	}
	app, err := filepath.EvalSymlinks(app)
	if err != nil {
		t.Fatal(err)
	}
	store := &memoryStore{value: DefaultSettings()}
	svc := NewService(store, t.TempDir())
	svc.platform = "darwin"
	svc.run = func(context.Context, string, ...string) ([]byte, error) { return []byte(app), nil }
	picked, err := svc.PickApplication(context.Background())
	if err != nil || picked == nil || picked.Builtin != "vscode" {
		t.Fatalf("preset: %+v %v", picked, err)
	}
	edit := DefaultSettings()
	edit.Default = Method{OpenWith: "custom", CustomAppPath: app}
	edit.CustomApps = []Application{*picked}
	saved, err := svc.Save(context.Background(), edit)
	if err != nil || saved.Default.OpenWith != "vscode" || saved.Default.CustomAppPath != "" || len(saved.CustomApps) != 0 {
		t.Fatalf("preset persisted as custom: %+v %v", saved, err)
	}
}
