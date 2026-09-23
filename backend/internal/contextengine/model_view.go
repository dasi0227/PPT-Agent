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
			case "project_id", "run_id", "thread_id", "loop_id", "context_id", "created_by_run", "approval_id", "approved_content_hash", "plan_id", "created_at", "updated_at", "rendered_at", "produced_at", "schema_version", "version", "local_path", "open_url", "source_ref", "estimated_tokens", "revision":
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

// ModelResource does not change the resource hash used by optimistic writes.
func ModelResource(raw []byte) (any, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return stripModelMetadata(value), nil
}

func ModelOutline(pack ContextPack) any {
	value := ModelValue(pack.Outline.Outline)
	ordinals := map[string]int{}
	for index, loc := range pptspec.FlattenOutline(pack.Outline.Outline) {
		ordinals[loc.Slide.SlideID] = index + 1
	}
	var visit func(any)
	visit = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if id, ok := v["slide_id"].(string); ok {
				v["ordinal"] = ordinals[id]
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

// ModelSections is the common input to full compilation and incremental
// assembly. Section names are stable; absent data is represented explicitly.
func ModelSections(pack ContextPack) map[string]any {
	sections := map[string]any{
		"run_command":     ModelValue(map[string]any{"scope": pack.Command.Scope, "mode": pack.Command.Mode, "options": pack.Command.Options}),
		"project_context": map[string]any{"title": pack.Project.Title, "manifest": ModelValue(pack.PresentationManifest.Manifest)},
		"outline":         ModelOutline(pack),
		"target_context":  nil, "related_context": nil, "design_context": ModelValue(pack.Design.Design),
		"theme_context": nil, "available_resources": map[string]any{"components": pack.Components, "skills": pack.Skills},
	}
	pages := map[string]map[string]any{}
	for _, summary := range pack.Outline.Summaries {
		pages[summary.ID] = map[string]any{"key_message": summary.KeyMessage, "materialization_state": summary.State}
	}
	for _, summary := range pack.RelatedSlides {
		if pages[summary.ID] == nil {
			pages[summary.ID] = map[string]any{"key_message": summary.KeyMessage, "materialization_state": summary.State}
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
		if pack.Target.SlideSpec != nil {
			pages[id]["spec_content_hash"] = pptspec.ResourceHash(*pack.Target.SlideSpec)
		}
		pages[id]["materialization"] = ModelValue(pack.Target.Materialization)
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
	if theme := pack.Theme; theme != nil {
		sections["theme_context"] = map[string]any{"name": theme.Name, "description": theme.Description, "tokens": theme.Tokens, "allowed_selectors": theme.AllowedSelectors}
	}
	return sections
}

// RefreshPageContext updates the canonical in-memory view after durable writes
// and renders. Only changed pages need disk reads unless deck dependencies changed.
func RefreshPageContext(pack *ContextPack, workDir string, touched map[string]bool, all bool) {
	previous := map[string]SlideSummary{}
	for _, summary := range pack.Outline.Summaries {
		previous[summary.ID] = summary
	}
	current := map[string]SlideSummary{}
	summaries := []SlideSummary{}
	for _, loc := range pptspec.FlattenOutline(pack.Outline.Outline) {
		id := loc.Slide.SlideID
		old, exists := previous[id]
		summary := slideSummary(loc, pptspec.SlideSpec{}, false)
		summary.KeyMessage, summary.State = old.KeyMessage, old.State
		if all || touched[id] || !exists {
			var slide pptspec.SlideSpec
			raw, err := os.ReadFile(filepath.Join(workDir, filepath.FromSlash(model.SlideSpecPath(id))))
			ready := err == nil && json.Unmarshal(raw, &slide) == nil
			summary = slideSummary(loc, slide, ready)
			if pack.Design.Design != nil {
				summary.State = loadMaterializationState(workDir, id, pack.PresentationManifest.Manifest, pack.Outline.Outline, slide, *pack.Design.Design)
			}
			if len(pack.Target.SlideIDs) == 1 && pack.Target.SlideIDs[0] == id {
				pack.Target.SlideSpec = nil
				if ready {
					pack.Target.SlideSpec = &slide
				}
				pack.Target.Materialization = &pptspec.Materialization{State: summary.State}
				pack.Target.SlideHTMLSummary = nil
				if html, _, err := (SlideHTMLSummaryLoader{}).Load(filepath.Join(workDir, filepath.FromSlash(model.SlideHTMLPath(id)))); err == nil {
					pack.Target.SlideHTMLSummary = &html
				}
			}
		}
		current[id] = summary
		summaries = append(summaries, summary)
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
		pack.Target.SlideSpec, pack.Target.SlideHTMLSummary, pack.Target.Materialization = nil, nil, nil
		pack.Target.SlideHTML = ""
	}
}
