package contextengine

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	pptspec "github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

// ModelValue projects known structured runtime data, never arbitrary user JSON
// documents. Text/code values are kept verbatim; routing and audit stay local.
func ModelValue(value any) any {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var out any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&out) != nil {
		return nil
	}
	return stripModelMetadata(out)
}

func stripModelMetadata(value any) any {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			switch key {
			case "image_path", "artifact_hash", "before_hash", "after_hash", "ok", "content_hash", "source_hash", "expected_hash", "spec_content_hash", "project_id", "run_id", "thread_id", "loop_id", "context_id", "created_by_run", "approval_id", "approved_content_hash", "plan_id", "created_at", "updated_at", "rendered_at", "produced_at", "schema_version", "version", "local_path", "open_url", "source_ref", "revision":
				delete(v, key)
			default:
				v[key] = stripModelMetadata(item)
			}
		}
	case []any:
		for i := range v {
			v[i] = stripModelMetadata(v[i])
		}
	}
	return value
}

// RefreshPageContext updates the canonical in-memory view after durable writes
// Only changed pages need disk reads unless deck dependencies changed.
func RefreshPageContext(pack *ContextPack, workDir string, touched map[string]bool, all bool) {
	previous := map[string]SlideSummary{}
	for _, summary := range pack.Outline.Summaries {
		previous[summary.ID] = summary
	}
	summaries := []SlideSummary{}
	entries, collectionErr := pptspec.ReadCollection(func(path string) ([]byte, error) {
		return os.ReadFile(filepath.Join(workDir, path))
	})
	for _, loc := range pptspec.FlattenOutline(pack.Outline.Outline) {
		id := loc.Slide.ID
		old, exists := previous[id]
		summary := slideSummary(loc, pptspec.SlideSpec{}, false)
		summary.Core, summary.State = old.Core, old.State
		summary.Purpose, summary.ContentType = old.Purpose, old.ContentType
		if all || touched[id] || !exists {
			var slide pptspec.SlideSpec
			raw, present := entries[id]
			ready := collectionErr == nil && present && json.Unmarshal(raw, &slide) == nil
			summary = slideSummary(loc, slide, ready)
			summary.State = loadHTMLState(workDir, id)
		}
		summaries = append(summaries, summary)
	}
	pack.Outline.Summaries = summaries
}
