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
		{"system", []string{resolved}}, {"custom", []string{"-a", app, resolved}},
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
