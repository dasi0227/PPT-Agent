package contextengine

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
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
			case "image_path", "artifact_hash", "before_hash", "after_hash", "ok", "content_hash", "source_hash", "expected_hash", "spec_content_hash", "project_id", "run_id", "thread_id", "loop_id", "context_id", "created_by_run", "approval_id", "approved_content_hash", "plan_id", "created_at", "updated_at", "rendered_at", "produced_at", "schema_version", "version", "local_path", "open_url", "source_ref", "estimated_tokens", "revision":
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

func ModelOutline(pack ContextPack) any {
	value := ModelValue(pack.Outline.Outline)
	ordinals := map[string]int{}
	for index, loc := range pptspec.FlattenOutline(pack.Outline.Outline) {
		ordinals[loc.Slide.ID] = index + 1
	}
	var visit func(any)
	visit = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if id, ok := v["id"].(string); ok {
				if ordinal, exists := ordinals[id]; exists {
					v["ordinal"] = ordinal
				}
			}
			for _, child := range v {
				visit(child)
			}
		case []any:
			for _, child := range v {
				visit(child)
			}
		}
	}
	visit(value)
	return value
}

// ModelSections projects the context used by incremental request assembly.
// Section names are stable; absent data is represented explicitly.
func ModelSections(pack ContextPack) map[string]any {
	sections := map[string]any{
		"run_command":     ModelValue(map[string]any{"scope": pack.Command.Scope, "mode": pack.Command.Mode}),
		"project_context": map[string]any{"title": pack.Project.Title, "manifest": ModelValue(pack.PresentationManifest.Manifest)},
		"outline":         ModelOutline(pack),
		"target_context":  nil, "related_context": nil, "design_context": ModelValue(pack.Design.Design),
		"available_skills":     pack.Skills,
		"available_components": pack.Components,
	}
	pages := map[string]map[string]any{}
	pageSummary := func(summary SlideSummary) map[string]any {
		content := map[string]any{"core": summary.Core, "html_state": summary.State}
		if summary.Purpose != "" {
			content["purpose"] = summary.Purpose
		}
		if summary.ContentType != "" {
			content["content_type"] = summary.ContentType
		}
		return content
	}
	for _, summary := range pack.Outline.Summaries {
		pages[summary.ID] = pageSummary(summary)
	}
	for _, summary := range pack.RelatedSlides {
		if pages[summary.ID] == nil {
			pages[summary.ID] = pageSummary(summary)
		}
	}
	if len(pack.Target.SlideIDs) > 0 {
		sections["target_context"] = map[string]any{"slide_ids": pack.Target.SlideIDs}
	}
	if len(pack.Target.SlideIDs) == 1 {
		id := pack.Target.SlideIDs[0]
		if pages[id] == nil {
			pages[id] = map[string]any{}
		}
		pages[id]["spec"] = ModelValue(pack.Target.SlideSpec)
		pages[id]["html_summary"] = ModelValue(pack.Target.SlideHTMLSummary)
	}
	for id, content := range pages {
		sections["page/"+id] = content
	}
	if len(pack.RelatedSlides) > 0 {
		ids := make([]string, 0, len(pack.RelatedSlides))
		for _, slide := range pack.RelatedSlides {
			ids = append(ids, slide.ID)
		}
		sections["related_context"] = ids
	}
	for id, changes := range referenceChanges(pack) {
		sections["html_reference_changes/"+id] = changes
	}
	return sections
}

// RefreshPageContext updates the canonical in-memory view after durable writes
// Only changed pages need disk reads unless deck dependencies changed.
func RefreshPageContext(pack *ContextPack, workDir string, touched map[string]bool, all bool) {
	previous := map[string]SlideSummary{}
	for _, summary := range pack.Outline.Summaries {
		previous[summary.ID] = summary
	}
	current := map[string]SlideSummary{}
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
			if ready {
				setGenerationInputs(pack, id, slide)
			} else {
				delete(pack.GenerationInputs, id)
			}
			if len(pack.Target.SlideIDs) == 1 && pack.Target.SlideIDs[0] == id {
				pack.Target.SlideSpec = nil
				if ready {
					pack.Target.SlideSpec = &slide
				}
				pack.Target.SlideHTMLSummary = nil
				if html, _, err := (SlideHTMLSummaryLoader{}).Load(filepath.Join(workDir, filepath.FromSlash(model.SlideHTMLPath(id)))); err == nil {
					pack.Target.SlideHTMLSummary = &html
				}
			}
		}
		current[id] = summary
		summaries = append(summaries, summary)
	}
	for id := range pack.GenerationInputs {
		if _, exists := current[id]; !exists {
			delete(pack.GenerationInputs, id)
		}
	}
	for id := range pack.GenerationBaselines {
		if _, exists := current[id]; !exists {
			delete(pack.GenerationBaselines, id)
		}
	}
	pack.Outline.Summaries = summaries
	related := []SlideSummary{}
	for _, old := range pack.RelatedSlides {
		if latest, ok := current[old.ID]; ok {
			related = append(related, latest)
		}
	}
	pack.RelatedSlides = related
	targetIDs := []string{}
	for _, id := range pack.Target.SlideIDs {
		if _, ok := current[id]; ok {
			targetIDs = append(targetIDs, id)
		}
	}
	pack.Target.SlideIDs = targetIDs
	if len(targetIDs) == 0 {
		pack.Target.SlideSpec, pack.Target.SlideHTMLSummary = nil, nil
	}
}
