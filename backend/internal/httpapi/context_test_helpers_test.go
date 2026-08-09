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
		sections = []spec.Section{{ID: "section-main", Title: "Main", Purpose: "Main test section", Subsections: []spec.Subsection{}}}
	}
	order := make([]string, 0, len(slides))
	deck := spec.Outline{
		SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, Title: "Test deck",
		Goal: "test goal", Audience: "test audience", Language: "zh-CN", Positioning: "test thesis",
		Constraints: spec.Constraints{MustInclude: []string{}, MustAvoid: []string{}, StyleLimits: []string{}, ContentLimits: []string{}},
		Sections:    sections, SlideOrder: order, CreatedAt: 1, UpdatedAt: 1,
	}
	for _, sl := range slides {
		order = append(order, sl.ID)
		slideSpec := spec.SlideSpec{
			SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID, SlideID: sl.ID,
			SectionID: "section-main",
			Role:      "evidence", Title: sl.Title, KeyMessage: sl.Title + " key message",
			Elements: []spec.Element{{Type: "text", Intent: sl.Title + " summary"}},
			Layout:   sl.Layout, CreatedAt: 1, UpdatedAt: 1,
		}
		writeTestJSON(t, filepath.Join(workDir, model.SlideSpecPath(sl.ID)), slideSpec)
	}
	deck.SlideOrder = order
	writeTestJSON(t, filepath.Join(workDir, "outline.json"), deck)
	writeTestJSON(t, filepath.Join(workDir, "design.json"), spec.Design{
		SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID,
		Theme:     "swiss-modern",
		Direction: "test direction",
		Density:   "medium",
		Chrome: []spec.ChromeItem{
			{Type: "page_number", Placement: "bottom-right", Style: "tiny muted mono counter"},
		},
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
