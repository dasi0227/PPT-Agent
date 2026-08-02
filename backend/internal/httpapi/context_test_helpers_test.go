package httpapi_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

// writeV3ContextSources keeps production-path E2E fixtures on the authoritative
// WorkSpec + Spec v3 contract.
func writeV3ContextSources(t *testing.T, workDir, projectID string, slides []model.Slide) {
	t.Helper()
	sections := []spec.Section{}
	if len(slides) > 0 {
		sections = []spec.Section{{ID: "section-main", Number: "1", Title: "Main", Subsections: []spec.Subsection{}}}
	}
	order := make([]string, 0, len(slides))
	deck := spec.Outline{
		SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Title: "Test deck",
		Goal: "test goal", Audience: "test audience", Language: "zh-CN", CoreThesis: "test thesis",
		NarrativeArc: "start → finish", Sections: sections, SlideOrder: order, CreatedAt: 1, UpdatedAt: 1,
	}
	for _, sl := range slides {
		order = append(order, sl.ID)
		slideSpec := spec.SlideSpec{
			SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, SlideID: sl.ID,
			SourceOutlineRevision: 1, SectionID: "section-main",
			Role: "evidence", Title: sl.Title, KeyMessage: sl.Title + " key message",
			Content:      spec.Content{Summary: sl.Title + " summary", Points: []string{"point"}},
			VisualIntent: spec.VisualIntent{Archetype: sl.Layout, Description: "test visual", AssetQueries: []string{}},
			CreatedAt:    1, UpdatedAt: 1,
		}
		writeTestJSON(t, filepath.Join(workDir, model.SlideSpecPath(sl.ID)), slideSpec)
	}
	deck.SlideOrder = order
	writeTestJSON(t, filepath.Join(workDir, "outline.json"), deck)
	writeTestJSON(t, filepath.Join(workDir, "design.json"), spec.Design{
		SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID,
		Canvas:  spec.CanvasSpec{Width: 1600, Height: 900, Ratio: "16:9"},
		Palette: []string{"#000000", "#FFFFFF"},
		Typography: spec.TypographySpec{
			Display: spec.FontSpec{Family: "Inter", Weight: 700},
			Body:    spec.FontSpec{Family: "Inter", Weight: 400},
			Utility: spec.FontSpec{Family: "Inter", Weight: 500},
		},
		Spacing: spec.SpacingSpec{Unit: 8}, Radius: spec.RadiusSpec{Card: 12},
		Shadows:      spec.ShadowSpec{Card: "0 8px 24px rgba(0,0,0,.2)"},
		LayoutSystem: spec.LayoutSystem{Grid: "12-col", Rhythm: "8", Density: "medium"},
		Signature:    "test signature", Motion: spec.MotionSpec{Policy: "restrained"},
		CreatedAt: 1, UpdatedAt: 1,
	})
}

func writeTestJSON(t *testing.T, path string, value any) {
	t.Helper()
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
