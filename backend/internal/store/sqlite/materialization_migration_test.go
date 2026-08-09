package sqlite

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func TestMaterializationMigrationWritesFileAndDropsBackup(t *testing.T) {
	root := t.TempDir()
	db, cleanup, err := Open(&config.Config{DBPath: filepath.Join(root, "migration.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := Migrate(db, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	workDir := filepath.Join(root, "project")
	if err := db.Exec(`
		INSERT INTO projects(id,title,work_dir,theme,status,layout_version,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?)
	`, "p1", "Deck", workDir, "default", "draft", 3, 1, 10).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO slides(id,project_id,current_version) VALUES(?,?,?)`, "s1", "p1", 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`
		INSERT INTO legacy_slide_materializations(
			slide_id,project_id,html_revision,source_outline_revision,
			source_spec_revision,source_design_revision
		) VALUES(?,?,?,?,?,?)
	`, "s1", "p1", 4, 2, 3, 2).Error; err != nil {
		t.Fatal(err)
	}
	writeMaterializationFixture(t, workDir, "outline.json", spec.Outline{
		SchemaVersion: spec.SchemaVersion, Revision: 2, ProjectID: "p1",
		Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN",
		Requirements: []string{}, Prohibitions: []string{},
		Sections:   []spec.Section{{ID: "sec", Title: "Main", Purpose: "Main section", Subsections: []spec.Subsection{}}},
		SlideOrder: []string{"s1"}, CreatedAt: 1, UpdatedAt: 2,
	})
	writeMaterializationFixture(t, workDir, "design.json", spec.Design{
		SchemaVersion: spec.SchemaVersion, Revision: 2, ProjectID: "p1",
		Theme: "swiss-modern", Direction: "test", Density: "medium",
		Chrome: []spec.ChromeItem{}, CreatedAt: 1, UpdatedAt: 2,
	})
	writeMaterializationFixture(t, workDir, model.SlideSpecPath("s1"), spec.SlideSpec{
		SchemaVersion: spec.SchemaVersion, Revision: 3, ProjectID: "p1", SlideID: "s1",
		SectionID: "sec", Role: "evidence", Title: "Title", KeyMessage: "Message",
		Elements:  []spec.Element{{Type: "text", Intent: "Explain the message"}},
		CreatedAt: 1, UpdatedAt: 3,
	})
	htmlPath := filepath.Join(workDir, model.SlideHTMLPath("s1"))
	if err := os.WriteFile(htmlPath, []byte("<!doctype html><html><body>slide</body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MigrateMaterializations(db, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	record, err := spec.ReadMaterialization(filepath.Join(workDir, model.SlideMaterializationPath("s1")))
	if err != nil {
		t.Fatal(err)
	}
	if record.Artifact.Revision != 4 || record.Source.Outline != 2 ||
		record.Source.Spec != 3 || record.Source.Design != 2 {
		t.Fatalf("materialization=%+v", record)
	}
	if db.Migrator().HasTable("legacy_slide_materializations") {
		t.Fatal("legacy backup table remains after successful migration")
	}
}

func writeMaterializationFixture(t *testing.T, root, relative string, value any) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}
