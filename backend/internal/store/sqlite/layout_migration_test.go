package sqlite

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func TestProjectLayoutMigrationIsAtomicAndOneWay(t *testing.T) {
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
	writeLegacyProject(t, workDir, false)
	insertLayoutV1Project(t, db, "p1", workDir)
	if err := MigrateProjectLayouts(db, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"outline.json", "design.json", model.SlideSpecPath("slide-01"),
		"common/base.css", "common/tokens.css",
		model.OutlineVersionSnapshot(2), model.SlideSpecVersionSnapshot("slide-01", 3),
		model.SlideHTMLVersionSnapshot("slide-01", 1),
	} {
		if _, err := os.Stat(filepath.Join(workDir, filepath.FromSlash(path))); err != nil {
			t.Fatalf("new resource missing %s: %v", path, err)
		}
	}
	for _, path := range []string{
		"deck.json", "design/design-spec.json", "slides/slide-01/slide.json",
		"versions/blueprint-deck/v2.json",
		"versions/blueprint-slide-slide-01/v3.json",
		"versions/presentation-slide-slide-01/v1.html",
	} {
		if _, err := os.Stat(filepath.Join(workDir, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("legacy resource remains %s: %v", path, err)
		}
	}
	var saved spec.SlideSpec
	raw, _ := os.ReadFile(filepath.Join(workDir, model.SlideSpecPath("slide-01")))
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.SchemaVersion != spec.SchemaVersion || saved.ProjectID != "p1" ||
		saved.SlideID != "slide-01" || saved.SourceOutlineRevision != 2 {
		t.Fatalf("migrated metadata=%+v", saved)
	}
	var row struct {
		LayoutVersion int    `gorm:"column:layout_version"`
		OutlinePath   string `gorm:"column:outline_path"`
	}
	if err := db.Table("projects").Where("id = ?", "p1").Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.LayoutVersion != currentProjectLayoutVersion || row.OutlinePath != "outline.json" {
		t.Fatalf("database layout not migrated: %+v", row)
	}
	var versions []layoutVersionRow
	if err := db.Table("versions").Order("id").Find(&versions).Error; err != nil {
		t.Fatal(err)
	}
	wantVersions := map[string]layoutVersionRow{
		"v-html": {
			TargetID:     model.SlideHTMLVersionTarget("p1", "slide-01"),
			SnapshotPath: model.SlideHTMLVersionSnapshot("slide-01", 1),
		},
		"v-outline": {
			TargetID:     model.OutlineVersionTarget("p1"),
			SnapshotPath: model.OutlineVersionSnapshot(2),
		},
		"v-spec": {
			TargetID:     model.SlideSpecVersionTarget("p1", "slide-01"),
			SnapshotPath: model.SlideSpecVersionSnapshot("slide-01", 3),
		},
	}
	for _, version := range versions {
		want, exists := wantVersions[version.ID]
		if !exists {
			continue
		}
		if version.TargetID != want.TargetID || version.SnapshotPath != want.SnapshotPath {
			t.Fatalf("version %s not migrated: %+v", version.ID, version)
		}
		delete(wantVersions, version.ID)
	}
	if len(wantVersions) != 0 {
		t.Fatalf("version rows missing after migration: %v", wantVersions)
	}
}

func TestProjectLayoutMigrationFailureLeavesLegacyLayoutUntouched(t *testing.T) {
	root := t.TempDir()
	db, cleanup, err := Open(&config.Config{DBPath: filepath.Join(root, "rollback.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := Migrate(db, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	workDir := filepath.Join(root, "project")
	writeLegacyProject(t, workDir, true)
	insertLayoutV1Project(t, db, "p1", workDir)
	if err := MigrateProjectLayouts(db, zap.NewNop()); err == nil {
		t.Fatal("mixed old/new layout was accepted")
	}
	if _, err := os.Stat(filepath.Join(workDir, "deck.json")); err != nil {
		t.Fatalf("legacy outline was changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "outline.json")); err != nil {
		t.Fatalf("pre-existing new outline was changed: %v", err)
	}
	var layoutVersion int
	if err := db.Raw("SELECT layout_version FROM projects WHERE id = ?", "p1").Scan(&layoutVersion).Error; err != nil {
		t.Fatal(err)
	}
	if layoutVersion != 1 {
		t.Fatalf("failed migration updated database version: %d", layoutVersion)
	}
}

func insertLayoutV1Project(t *testing.T, db *gorm.DB, projectID, workDir string) {
	t.Helper()
	if err := db.Exec(`INSERT INTO projects(
		id,title,work_dir,theme,status,design_path,outline_path,
		outline_revision,design_revision,layout_version,created_at,updated_at
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		projectID, "Legacy", workDir, "default", "draft", "design/design-spec.json", "deck.json",
		2, 1, 1, 1, 2,
	).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO slides(
		id,project_id,position,layout,title,spec_path,html_path,current_version,
		spec_revision,html_revision,source_outline_revision,source_spec_revision,source_design_revision
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		"slide-01", projectID, 0, "cover", "Legacy slide", "slides/slide-01/slide.json",
		"slides/slide-01/index.html", 1, 3, 1, 2, 3, 1,
	).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO versions(
		id,target_type,target_id,version_no,snapshot_path,run_id,created_at
	) VALUES
		(?,?,?,?,?,?,?),
		(?,?,?,?,?,?,?),
		(?,?,?,?,?,?,?)`,
		"v-outline", "outline", "project/"+projectID+"/blueprint-deck", 2,
		"versions/blueprint-deck/v2.json", nil, 2,
		"v-spec", "slide_spec", "project/"+projectID+"/blueprint-slide-slide-01", 3,
		"versions/blueprint-slide-slide-01/v3.json", nil, 2,
		"v-html", "slide_html", "project/"+projectID+"/slide-slide-01", 1,
		"versions/presentation-slide-slide-01/v1.html", nil, 2,
	).Error; err != nil {
		t.Fatal(err)
	}
}

func writeLegacyProject(t *testing.T, workDir string, includeNewOutline bool) {
	t.Helper()
	outline := deckModelForMigration()
	design := designModelForMigration()
	slide := slideModelForMigration()
	write := func(relative string, value any) {
		path := filepath.Join(workDir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(value)
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("deck.json", outline)
	write("design/design-spec.json", design)
	write("slides/slide-01/slide.json", slide)
	write("versions/blueprint-deck/v2.json", outline)
	write("versions/blueprint-slide-slide-01/v3.json", slide)
	htmlPath := filepath.Join(workDir, "versions/presentation-slide-slide-01/v1.html")
	if err := os.MkdirAll(filepath.Dir(htmlPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(htmlPath, []byte("<!doctype html><html><body>Legacy</body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if includeNewOutline {
		write("outline.json", outline)
	}
}

func deckModelForMigration() map[string]any {
	return map[string]any{
		"schema_version": "2.0", "revision": 2, "title": "Legacy", "goal": "Goal",
		"audience": "Audience", "language": "zh-CN", "core_thesis": "Thesis",
		"narrative_arc": "Arc",
		"sections": []any{map[string]any{
			"id": "section-1", "number": "1", "title": "Section", "subsections": []any{},
		}},
		"slide_order": []string{"slide-01"}, "created_at": 1, "updated_at": 2,
	}
}

func designModelForMigration() map[string]any {
	return map[string]any{
		"schema_version": "2.0", "revision": 1,
		"canvas":  map[string]any{"width": 1440, "height": 810, "ratio": "16:9"},
		"palette": []string{"#000000", "#FFFFFF"},
		"typography": map[string]any{
			"display": map[string]any{"family": "Arial", "weight": 700},
			"body":    map[string]any{"family": "Arial", "weight": 400},
			"utility": map[string]any{"family": "Arial", "weight": 500},
		},
		"spacing": map[string]any{"unit": 8}, "radius": map[string]any{"card": 8},
		"shadows":       map[string]any{"card": "none"},
		"layout_system": map[string]any{"grid": "12-col", "rhythm": "regular", "density": "medium"},
		"signature":     "legacy", "motion": map[string]any{"policy": "none"},
	}
}

func slideModelForMigration() map[string]any {
	return map[string]any{
		"schema_version": "2.0", "revision": 3, "slide_id": "slide-01",
		"section_id": "section-1", "role": "cover", "title": "Legacy slide",
		"key_message": "Legacy",
		"content":     map[string]any{"summary": "Legacy", "points": []string{}},
		"visual_intent": map[string]any{
			"archetype": "cover", "description": "Cover", "asset_queries": []string{},
		},
		"speaker_notes": "", "created_at": 1, "updated_at": 2,
	}
}
