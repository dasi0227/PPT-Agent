package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

type failingResourceStore struct {
	ResourceStore
	failUpdate bool
	failDelete bool
	failCreate bool
}

func (s *failingResourceStore) UpdateResource(c context.Context, r model.Resource) error {
	if s.failUpdate {
		return errors.New("update failed")
	}
	return s.ResourceStore.UpdateResource(c, r)
}
func (s *failingResourceStore) DeleteResource(c context.Context, kind, id string) error {
	if s.failDelete {
		return errors.New("delete failed")
	}
	return s.ResourceStore.DeleteResource(c, kind, id)
}
func (s *failingResourceStore) CreateResource(c context.Context, r model.Resource) error {
	if s.failCreate {
		return errors.New("create failed")
	}
	return s.ResourceStore.CreateResource(c, r)
}
func TestSnippetWritesAndDatabaseFailures(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	st := &failingResourceStore{ResourceStore: newMemoryResourceStore()}
	svc := NewResourceService(WorkRoot(root), st)
	v, err := svc.CreateSnippet(ctx, model.Resource{Name: "  Example  ", Description: "Description", Tags: []string{"other"}}, "exact\nbody\n")
	if err != nil {
		t.Fatal(err)
	}
	if v.Name != "Example" || v.Content != "exact\nbody\n" || v.ContentState != "ready" {
		t.Fatalf("snippet=%+v", v)
	}
	if _, err = svc.CreateSnippet(ctx, model.Resource{Name: "EXAMPLE", Description: "Duplicate"}, "body"); !errors.Is(err, store.ErrResourceConflict) {
		t.Fatal(err)
	}
	st.failUpdate = true
	if _, err = svc.WriteSnippet(ctx, v.ID, "replacement"); err == nil {
		t.Fatal("failed DB update accepted")
	}
	current, _ := svc.Snippet(ctx, v.ID)
	if current.Content != v.Content {
		t.Fatal("payload not restored")
	}
	st.failCreate = true
	if _, err = svc.CreateSnippet(ctx, model.Resource{Name: "Another", Description: "Description"}, "body"); err == nil {
		t.Fatal("failed insert accepted")
	}
	entries, _ := os.ReadDir(filepath.Join(root, "assets/snippets"))
	if len(entries) != 1 {
		t.Fatal("failed create left files")
	}
	path, _ := svc.payloadPath("snippet", v.ID)
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	st.failUpdate = false
	if _, err = svc.WriteSnippet(ctx, v.ID, "cannot write directory"); !errors.Is(err, ErrUnsafeRepositoryPath) {
		t.Fatal(err)
	}
	r, _ := st.GetResource(ctx, "snippet", v.ID)
	if r.Name != v.Name {
		t.Fatal("failed file write changed metadata")
	}
}
func TestResourceDeletionRollbackAndCrashRecovery(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	st := &failingResourceStore{ResourceStore: newMemoryResourceStore()}
	svc := NewResourceService(WorkRoot(root), st)
	registerFixture(t, root, st, "skill", "one", "One", "Description", "instructions")
	st.failDelete = true
	if err := svc.Delete(ctx, "skill", "one"); err == nil {
		t.Fatal("failed delete accepted")
	}
	_, raw, path, state, err := svc.Inspect(ctx, "skill", "one")
	if err != nil || state.ContentState != "ready" || string(raw) != "instructions" {
		t.Fatal("delete did not restore content")
	}
	st.failDelete = false
	trash, _ := svc.trashDir("skill", "one")
	if err = os.MkdirAll(filepath.Dir(trash), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(filepath.Dir(path), trash); err != nil {
		t.Fatal(err)
	}
	if err = svc.RecoverDeletes(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatal("uncommitted delete not restored")
	}
	if err = os.Rename(filepath.Dir(path), trash); err != nil {
		t.Fatal(err)
	}
	if err = st.DeleteResource(ctx, "skill", "one"); err != nil {
		t.Fatal(err)
	}
	if err = svc.RecoverDeletes(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(trash); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("committed trash not cleaned")
	}
}

func TestSnippetFileWriteFailureLeavesDatabaseUntouched(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires filesystem permission enforcement")
	}
	ctx := context.Background()
	root := t.TempDir()
	st := newMemoryResourceStore()
	svc := NewResourceService(WorkRoot(root), st)
	v, err := svc.CreateSnippet(ctx, model.Resource{Name: "Writable", Description: "Description"}, "Original")
	if err != nil {
		t.Fatal(err)
	}
	path, err := svc.payloadPath("snippet", v.ID)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(path)
	if err = os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0755)
	if _, err = svc.WriteSnippet(ctx, v.ID, "Replacement"); err == nil {
		t.Fatal("unwritable directory accepted replacement")
	}
	got, err := svc.Snippet(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != v.Content || got.UpdatedAt != v.UpdatedAt {
		t.Fatalf("failed write changed resource: %+v", got)
	}
}

func TestExternalPayloadEditsDoNotRewriteMetadata(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	st := newMemoryResourceStore()
	svc := NewResourceService(WorkRoot(root), st)
	registerFixture(t, root, st, "snippet", "external", "Database name", "Database description", "Original")
	before, err := svc.Snippet(ctx, "external")
	if err != nil {
		t.Fatal(err)
	}
	path, _ := svc.payloadPath("snippet", "external")
	writeRepositoryFile(t, path, "---\nname: literal content\n---\nExternal edit\n")
	future := time.Now().Add(24 * time.Hour)
	if err = os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	after, err := svc.Snippet(ctx, "external")
	if err != nil {
		t.Fatal(err)
	}
	if after.Name != before.Name || after.Description != before.Description || after.UpdatedAt != before.UpdatedAt || after.Content == before.Content {
		t.Fatalf("external edit=%+v", after)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.WriteSnippet(ctx, "external", "Repaired"); err != nil {
		t.Fatal(err)
	}
	repaired, _ := svc.Snippet(ctx, "external")
	if repaired.ContentState != "ready" || repaired.Content != "Repaired" {
		t.Fatal("missing payload was not repaired")
	}
}
