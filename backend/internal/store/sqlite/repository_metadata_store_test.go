package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/store"
	"go.uber.org/zap"
)

func newRepositoryMetadataStoreForTest(t *testing.T) *Store {
	t.Helper()
	db, cleanup, err := Open(&config.Config{DBPath: filepath.Join(t.TempDir(), "metadata.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	value, err := NewStore(db, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestRepositoryMetadataDefaultsAndMutations(t *testing.T) {
	ctx := context.Background()
	st := newRepositoryMetadataStoreForTest(t)

	tags, err := st.ListResourceTagKeys(ctx, "theme", "swiss-modern")
	if err != nil || len(tags) != 1 || tags[0] != "minimal" {
		t.Fatalf("default theme tags=%v err=%v", tags, err)
	}
	if err := st.ReplaceResourceTagKeys(ctx, "theme", "swiss-modern", []string{"business", "other"}); err != nil {
		t.Fatal(err)
	}
	tags, err = st.ListResourceTagKeys(ctx, "theme", "swiss-modern")
	if err != nil || len(tags) != 2 || tags[0] != "business" || tags[1] != "other" {
		t.Fatalf("updated theme tags=%v err=%v", tags, err)
	}
	if err := st.ReplaceResourceTagKeys(ctx, "theme", "swiss-modern", []string{"workflow"}); !errors.Is(err, store.ErrTagNotFound) {
		t.Fatalf("cross-scope tag error=%v", err)
	}

	if err := st.SetResourceDisabled(ctx, "component", "feature-card", true, 10); err != nil {
		t.Fatal(err)
	}
	disabled, err := st.GetResourceDisabled(ctx, "component", "feature-card")
	if err != nil || !disabled {
		t.Fatalf("disabled=%v err=%v", disabled, err)
	}
	if err := st.DeleteResourceMetadata(ctx, "component", "feature-card"); err != nil {
		t.Fatal(err)
	}
	tags, err = st.ListResourceTagKeys(ctx, "component", "feature-card")
	if err != nil || len(tags) != 0 {
		t.Fatalf("deleted component tags=%v err=%v", tags, err)
	}
	disabled, err = st.GetResourceDisabled(ctx, "component", "feature-card")
	if err != nil || disabled {
		t.Fatalf("deleted component state=%v err=%v", disabled, err)
	}
}
