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
		saved.SlideID != "slide-01" || saved.Layout != "cover" ||
		len(saved.Elements) != 1 || saved.Elements[0].Type != "text" {
		t.Fatalf("migrated metadata=%+v", saved)
	}
	var row struct {
		LayoutVersion int `gorm:"column:layout_version"`
	}
	if err := db.Table("projects").Where("id = ?", "p1").Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.LayoutVersion != currentProjectLayoutVersion {
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

func TestProjectLayoutV2NormalizesSlideSpecInPlace(t *testing.T) {
	root := t.TempDir()
	db, cleanup, err := Open(&config.Config{DBPath: filepath.Join(root, "v2.db")}, zap.NewNop())
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
	`, "p1", "Deck", workDir, "default", "draft", 2, 1, 2).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO slides(id,project_id) VALUES(?,?)`, "slide-01", "p1").Error; err != nil {
		t.Fatal(err)
	}
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
	write("outline.json", spec.Outline{
		SchemaVersion: spec.SchemaVersion, Revision: 2, ProjectID: "p1",
		Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN",
		Requirements: []string{}, Prohibitions: []string{},
		Sections:   []spec.Section{{ID: "section-1", Title: "Section", Purpose: "Main", Subsections: []spec.Subsection{}}},
		SlideOrder: []string{"slide-01"}, CreatedAt: 1, UpdatedAt: 2,
	})
	write("design.json", spec.Design{
		SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: "p1",
		Theme: "swiss-modern", Direction: "test", Density: "medium",
		Chrome: []spec.ChromeItem{}, CreatedAt: 1, UpdatedAt: 2,
	})
	write(model.SlideSpecPath("slide-01"), slideModelForMigration())

	if err := MigrateProjectLayouts(db, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	var saved spec.SlideSpec
	raw, err := os.ReadFile(filepath.Join(workDir, model.SlideSpecPath("slide-01")))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Layout != "cover" || len(saved.Elements) != 1 ||
		saved.Elements[0].Type != "text" || saved.Elements[0].Intent != "Legacy" {
		t.Fatalf("normalized spec=%+v", saved)
	}
}

func TestProjectLayoutV4FlattensOutlineConstraintsInPlace(t *testing.T) {
	root := t.TempDir()
	db, cleanup, err := Open(&config.Config{DBPath: filepath.Join(root, "v4.db")}, zap.NewNop())
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
	`, "p1", "Deck", workDir, "default", "draft", 4, 1, 2).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO slides(id,project_id) VALUES(?,?)`, "slide-01", "p1").Error; err != nil {
		t.Fatal(err)
	}
	write := func(relative string, value any) {
		path := filepath.Join(workDir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(legacyV4Resource(t, value))
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	outline := spec.Outline{
		SchemaVersion: spec.SchemaVersion, Revision: 2, ProjectID: "p1",
		Title: "Deck", Goal: "Goal", Audience: "Audience", Language: "zh-CN",
		Requirements: []string{"Include trend", "Formal tone", "At most 12 pages"},
		Prohibitions: []string{"Fabricated data"},
		Sections:     []spec.Section{{ID: "section-1", Title: "Section", Purpose: "Main", Subsections: []spec.Subsection{}}},
		SlideOrder:   []string{"slide-01"}, CreatedAt: 1, UpdatedAt: 2,
	}
	design := spec.Design{
		SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: "p1",
		Theme: "swiss-modern", Direction: "test", Density: "medium",
		Chrome: []spec.ChromeItem{}, CreatedAt: 1, UpdatedAt: 2,
	}
	slideSpec := spec.SlideSpec{
		SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: "p1", SlideID: "slide-01",
		SectionID: "section-1", Role: "content", Title: "Slide", KeyMessage: "Message",
		Elements: []spec.Element{{Type: "text", Intent: "Content"}}, CreatedAt: 1, UpdatedAt: 2,
	}
	write("outline.json", outline)
	write("design.json", design)
	write(model.SlideSpecPath("slide-01"), slideSpec)
	write(model.OutlineVersionSnapshot(2), outline)
	write(model.DesignVersionSnapshot(1), design)
	write(model.SlideSpecVersionSnapshot("slide-01", 1), slideSpec)
	outlineRaw, _ := os.ReadFile(filepath.Join(workDir, "outline.json"))
	designRaw, _ := os.ReadFile(filepath.Join(workDir, "design.json"))
	specRaw, _ := os.ReadFile(filepath.Join(workDir, model.SlideSpecPath("slide-01")))
	materialization := spec.MaterializationRecord{
		SchemaVersion: spec.SchemaVersion,
		Artifact: spec.MaterializationArtifact{
			Revision: 1,
			Hash:     "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		Source: spec.MaterializationSource{
			Outline: 2,
			Spec:    1,
			Design:  1,
			Hash:    spec.SourceHash(outlineRaw, specRaw, designRaw),
		},
		RenderedAt: 2,
	}
	materializationRaw, _ := json.Marshal(materialization)
	materializationPath := filepath.Join(workDir, model.SlideMaterializationPath("slide-01"))
	if err := os.WriteFile(materializationPath, materializationRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO versions(
		id,target_type,target_id,version_no,snapshot_path,run_id,created_at
	) VALUES
		(?,?,?,?,?,?,?),
		(?,?,?,?,?,?,?),
		(?,?,?,?,?,?,?)`,
		"v-outline", "outline", model.OutlineVersionTarget("p1"), 2, model.OutlineVersionSnapshot(2), nil, 2,
		"v-design", "design", model.DesignVersionTarget("p1"), 1, model.DesignVersionSnapshot(1), nil, 2,
		"v-spec", "slide_spec", model.SlideSpecVersionTarget("p1", "slide-01"), 1, model.SlideSpecVersionSnapshot("slide-01", 1), nil, 2,
	).Error; err != nil {
		t.Fatal(err)
	}

	if err := MigrateProjectLayouts(db, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{
		"outline.json", "design.json", model.SlideSpecPath("slide-01"),
		model.OutlineVersionSnapshot(2), model.DesignVersionSnapshot(1),
		model.SlideSpecVersionSnapshot("slide-01", 1),
	} {
		raw, err := os.ReadFile(filepath.Join(workDir, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatal(err)
		}
		if value["project_id"] != "p1" {
			t.Errorf("%s project_id=%v", relative, value["project_id"])
		}
		if _, exists := value["project"]; exists {
			t.Errorf("%s still contains obsolete project field", relative)
		}
	}
	migratedMaterialization, err := spec.ReadMaterialization(materializationPath)
	if err != nil {
		t.Fatal(err)
	}
	outlineRaw, _ = os.ReadFile(filepath.Join(workDir, "outline.json"))
	designRaw, _ = os.ReadFile(filepath.Join(workDir, "design.json"))
	specRaw, _ = os.ReadFile(filepath.Join(workDir, model.SlideSpecPath("slide-01")))
	if want := spec.SourceHash(outlineRaw, specRaw, designRaw); migratedMaterialization.Source.Hash != want {
		t.Fatalf("materialization source hash=%q want %q", migratedMaterialization.Source.Hash, want)
	}
	var migratedOutline spec.Outline
	if err := json.Unmarshal(outlineRaw, &migratedOutline); err != nil {
		t.Fatal(err)
	}
	if len(migratedOutline.Requirements) != 3 ||
		migratedOutline.Requirements[0] != "Include trend" ||
		migratedOutline.Requirements[1] != "Formal tone" ||
		migratedOutline.Requirements[2] != "At most 12 pages" ||
		len(migratedOutline.Prohibitions) != 1 ||
		migratedOutline.Prohibitions[0] != "Fabricated data" {
		t.Fatalf("migrated outline rules=%+v", migratedOutline)
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

func legacyV4Resource(t *testing.T, value any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if _, isOutline := result["sections"]; isOutline {
		result["constraints"] = map[string]any{
			"must_include":   []any{"Include trend"},
			"must_avoid":     []any{"Fabricated data"},
			"style_limits":   []any{"Formal tone"},
			"content_limits": []any{"At most 12 pages"},
		}
		delete(result, "requirements")
		delete(result, "prohibitions")
	}
	return result
}

func insertLayoutV1Project(t *testing.T, db *gorm.DB, projectID, workDir string) {
	t.Helper()
	if err := db.Exec(`INSERT INTO projects(
		id,title,work_dir,theme,status,layout_version,created_at,updated_at
	) VALUES(?,?,?,?,?,?,?,?)`,
		projectID, "Legacy", workDir, "default", "draft",
		1, 1, 2,
	).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO slides(
		id,project_id,current_version
	) VALUES(?,?,?)`,
		"slide-01", projectID, 1,
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
