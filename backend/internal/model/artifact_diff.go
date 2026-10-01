package model

import (
	"encoding/json"
	"fmt"
)

// ArtifactDiff is an immutable operation or terminal-run projection, persisted
// with its event rather than regenerated from later project contents.
type ArtifactDiff struct {
	Kind     string             `json:"kind"`
	Status   string             `json:"status"`
	Filename string             `json:"filename"`
	Fields   []FieldDiff        `json:"fields"`
	Hunks    []DiffHunk         `json:"hunks,omitempty"`
	Groups   []OutlineDiffGroup `json:"groups"`
	Error    string             `json:"error,omitempty"`
}

type FieldDiff struct {
	Field string         `json:"field"`
	Label string         `json:"label,omitempty"`
	Rows  []FieldDiffRow `json:"rows"`
}

type FieldDiffRow struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type DiffHunk struct {
	OldStart      int           `json:"old_start"`
	OldCount      int           `json:"old_count"`
	NewStart      int           `json:"new_start"`
	NewCount      int           `json:"new_count"`
	ContextBefore []TextDiffRow `json:"context_before,omitempty"`
	Rows          []TextDiffRow `json:"rows"`
}

type TextDiffRow struct {
	Kind    string `json:"kind"`
	Text    string `json:"text"`
	OldLine int    `json:"old_line,omitempty"`
	NewLine int    `json:"new_line,omitempty"`
}

type OutlineDiffGroup struct {
	Rows []OutlineDiffRow `json:"rows"`
}

type OutlineDiffRow struct {
	Kind  string `json:"kind"`
	Node  string `json:"node"`
	Depth int    `json:"depth"`
	Title string `json:"title"`
	Order string `json:"order,omitempty"`
}

func (d ArtifactDiff) Validate() error {
	if d.Kind != "unavailable" && d.Filename == "" {
		return fmt.Errorf("diff filename is required")
	}
	if d.Status != "added" && d.Status != "modified" && d.Status != "deleted" {
		return fmt.Errorf("invalid diff status")
	}
	switch d.Kind {
	case "outline":
		if d.Groups == nil {
			return fmt.Errorf("outline diff groups are required")
		}
		for _, group := range d.Groups {
			for _, row := range group.Rows {
				if row.Kind != "context" && row.Kind != "added" && row.Kind != "removed" {
					return fmt.Errorf("invalid outline diff row")
				}
				switch row.Node {
				case "chapter":
					if row.Depth != 0 {
						return fmt.Errorf("invalid chapter diff depth")
					}
				case "subchapter":
					if row.Depth != 1 {
						return fmt.Errorf("invalid subsection diff depth")
					}
				case "page", "purpose":
					if row.Depth != 1 && row.Depth != 2 {
						return fmt.Errorf("invalid outline leaf depth")
					}
				default:
					return fmt.Errorf("invalid outline diff node")
				}
			}
		}
	case "fields":
		if d.Fields == nil {
			return fmt.Errorf("field diff rows are required")
		}
		for _, field := range d.Fields {
			for _, row := range field.Rows {
				if (row.Kind != "added" && row.Kind != "removed") || !json.Valid([]byte(row.Value)) {
					return fmt.Errorf("invalid field diff row")
				}
			}
		}
	case "text":
		if d.Hunks == nil {
			return fmt.Errorf("text diff hunks are required")
		}
		for _, hunk := range d.Hunks {
			if hunk.OldStart < 0 || hunk.OldCount < 0 || hunk.NewStart < 0 || hunk.NewCount < 0 {
				return fmt.Errorf("invalid diff range")
			}
			for _, row := range hunk.ContextBefore {
				if row.Kind != "context" || row.OldLine <= 0 || row.NewLine <= 0 {
					return fmt.Errorf("collapsed context requires unchanged rows with both line numbers")
				}
			}
			for _, row := range hunk.Rows {
				if row.OldLine < 0 || row.NewLine < 0 {
					return fmt.Errorf("invalid diff line")
				}
				switch row.Kind {
				case "context":
					if row.OldLine == 0 || row.NewLine == 0 {
						return fmt.Errorf("context requires both line numbers")
					}
				case "removed":
					if row.OldLine == 0 || row.NewLine != 0 {
						return fmt.Errorf("removed row requires old line only")
					}
				case "added":
					if row.NewLine == 0 || row.OldLine != 0 {
						return fmt.Errorf("added row requires new line only")
					}
				default:
					return fmt.Errorf("invalid text diff row")
				}
			}
		}
	case "binary":
	case "unavailable":
		if d.Error == "" {
			return fmt.Errorf("diff error is required")
		}
	default:
		return fmt.Errorf("invalid diff kind")
	}
	return nil
}
