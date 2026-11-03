package sqlite

import (
	"context"
	"fmt"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestCurrentSchemaInitializationAndOldFormatRejection(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateProject(context.Background(), model.Project{ID: "p", Title: "Project", Theme: "theme", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := initializeSchema(s.db); err != nil {
		t.Fatal("repeat initialization", err)
	}
	if err := s.db.Exec(fmt.Sprintf("PRAGMA user_version=%d", schemaVersion-1)).Error; err != nil {
		t.Fatal(err)
	}
	if err := initializeSchema(s.db); err == nil {
		t.Fatal("old format accepted")
	}
	var title string
	if err := s.db.Raw("SELECT title FROM projects WHERE id='p'").Scan(&title).Error; err != nil || title != "Project" {
		t.Fatalf("format refusal changed data: %q %v", title, err)
	}
}
