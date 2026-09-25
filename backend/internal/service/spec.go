package service

import (
	"encoding/json"
	"os"

	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func defaultDesign() spec.Design {
	return spec.Design{
		Direction:         "",
		LayoutPreferences: []string{},
		Decorations:       spec.DefaultDecorations(),
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
