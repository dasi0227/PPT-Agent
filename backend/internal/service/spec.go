package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func defaultDesign(projectID string, now int64) spec.Design {
	return spec.Design{
		SchemaVersion: spec.SchemaVersion, Revision: 1, ProjectID: projectID,
		Theme:     "swiss-modern",
		Direction: "清晰、克制、结构化的通用商务演示",
		Density:   "medium",
		Chrome: []spec.ChromeItem{
			{Type: "page_number", Placement: "bottom-right", Style: "tiny muted mono counter"},
			{Type: "section_marker", Placement: "top-left", Style: "compact section label"},
		},
		CreatedAt: now, UpdatedAt: now,
	}
}

func atomicWrite(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".artifact-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(raw); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nonNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
