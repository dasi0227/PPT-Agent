package service

import (
	"encoding/json"
	"os"

	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func defaultDesign(projectID string, now int64) spec.Design {
	return spec.Design{
		SchemaVersion: spec.SchemaVersion, ProjectID: projectID,
		Direction:         "",
		LayoutPreferences: []string{},
		Decorations:       spec.DefaultDecorations(),
		CreatedAt:         now, UpdatedAt: now,
	}
}

func readJSON(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func mustJSON(v any) []byte {
	raw, _ := json.MarshalIndent(v, "", "  ")
	return raw
}
