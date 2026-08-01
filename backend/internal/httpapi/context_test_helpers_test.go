package httpapi_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// writeV2ContextSources keeps production-path E2E fixtures on the authoritative
// WorkSpec + Blueprint v2 contract used by Context Engineering v1.
func writeV2ContextSources(t *testing.T, workDir, projectID string, slides []model.Slide) {
	t.Helper()
	sections := []blueprint.Section{}
	if len(slides) > 0 {
		sections = []blueprint.Section{{ID: "section-main", Number: "1", Title: "Main", Subsections: []blueprint.Subsection{}}}
	}
	order := make([]string, 0, len(slides))
	deck := blueprint.Deck{
		SchemaVersion: blueprint.SchemaVersion, Revision: 1, ProjectID: projectID, Title: "Test deck",
		Goal: "test goal", Audience: "test audience", Language: "zh-CN", CoreThesis: "test thesis",
		NarrativeArc: "start → finish", Sections: sections, SlideOrder: order, CreatedAt: 1, UpdatedAt: 1,
	}
	for _, sl := range slides {
		order = append(order, sl.ID)
		bp := blueprint.Slide{
			SchemaVersion: blueprint.SchemaVersion, Revision: 1, SlideID: sl.ID, SectionID: "section-main",
			Role: "evidence", Title: sl.Title, KeyMessage: sl.Title + " key message",
			Content:      blueprint.Content{Summary: sl.Title + " summary", Points: []string{"point"}},
			VisualIntent: blueprint.VisualIntent{Archetype: sl.Layout, Description: "test visual", AssetQueries: []string{}},
			CreatedAt:    1, UpdatedAt: 1,
		}
		writeTestJSON(t, filepath.Join(workDir, "slides", sl.ID, "slide.json"), bp)
	}
	deck.SlideOrder = order
	writeTestJSON(t, filepath.Join(workDir, "deck.json"), deck)
	writeTestJSON(t, filepath.Join(workDir, "design", "design-spec.json"), blueprint.DesignSpec{
		SchemaVersion: blueprint.SchemaVersion, Revision: 1,
		Canvas:  blueprint.CanvasSpec{Width: 1600, Height: 900, Ratio: "16:9"},
		Palette: []string{"#000000"},
		Typography: blueprint.TypographySpec{
			Display: blueprint.FontSpec{Family: "Inter", Weight: 700},
			Body:    blueprint.FontSpec{Family: "Inter", Weight: 400},
			Utility: blueprint.FontSpec{Family: "Inter", Weight: 500},
		},
		Spacing: blueprint.SpacingSpec{Unit: 8}, Radius: blueprint.RadiusSpec{Card: 12},
		Shadows:      blueprint.ShadowSpec{Card: "0 8px 24px rgba(0,0,0,.2)"},
		LayoutSystem: blueprint.LayoutSystem{Grid: "12-col", Rhythm: "8", Density: "balanced"},
		Signature:    "test signature", Motion: blueprint.MotionSpec{Policy: "restrained"},
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
