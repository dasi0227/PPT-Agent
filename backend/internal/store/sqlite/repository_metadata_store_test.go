package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
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

func TestResourcesPersistMetadataAndTagsAtomically(t *testing.T) {
	ctx := context.Background()
	st := newRepositoryMetadataStoreForTest(t)
	original := model.Resource{Type: "component", ID: "card", Name: "Card", NormalizedName: "card", Description: "Description", Tags: []string{"card"}, CreatedAt: 1, UpdatedAt: 1}
	if err := st.CreateResource(ctx, original); err != nil {
		t.Fatal(err)
	}
	changed := original
	changed.Name = "Changed"
	changed.Disabled = true
	changed.Tags = []string{"workflow"}
	if err := st.UpdateResource(ctx, changed); !errors.Is(err, store.ErrTagNotFound) {
		t.Fatalf("invalid tag=%v", err)
	}
	got, err := st.GetResource(ctx, "component", "card")
	if err != nil || got.Name != original.Name || got.Disabled || len(got.Tags) != 1 || got.Tags[0] != "card" {
		t.Fatalf("failed transaction leaked: %+v %v", got, err)
	}
	changed.Tags = []string{"list"}
	if err := st.UpdateResource(ctx, changed); err != nil {
		t.Fatal(err)
	}
	got, err = st.GetResource(ctx, "component", "card")
	if err != nil || !got.Disabled || got.Name != "Changed" || got.Tags[0] != "list" {
		t.Fatalf("update=%+v %v", got, err)
	}
	if err = st.DeleteResource(ctx, "component", "card"); err != nil {
		t.Fatal(err)
	}
	var count int64
	st.db.Table("resource_tags").Where("resource_type = ? AND resource_id = ?", "component", "card").Count(&count)
	if count != 0 {
		t.Fatal("resource tags did not cascade")
	}
}
func TestOnlySnippetNamesAreUnique(t *testing.T) {
	st := newRepositoryMetadataStoreForTest(t)
	ctx := context.Background()
	for _, kind := range []string{"theme", "component", "skill", "snippet"} {
		r := model.Resource{Type: kind, ID: "one", Name: "Same", NormalizedName: "same", Description: "Description"}
		if err := st.CreateResource(ctx, r); err != nil {
			t.Fatal(err)
		}
		r.ID = "two"
		err := st.CreateResource(ctx, r)
		if kind == "snippet" {
			if !errors.Is(err, store.ErrResourceConflict) {
				t.Fatal(err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
}
