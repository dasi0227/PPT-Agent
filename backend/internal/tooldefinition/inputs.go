package tooldefinition

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"
)

const (
	MaxTextTitleRunes = 48
	MaxPolishRunes    = 8000
	MaxHandoffRunes   = 40000
)

func object(required []string, properties map[string]any) map[string]any {
	out := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) != 0 {
		out["required"] = required
	}
	return out
}

func stringSchema(minimum, maximum int) map[string]any {
	out := map[string]any{"type": "string"}
	if minimum > 0 {
		out["minLength"] = minimum
	}
	if maximum > 0 {
		out["maxLength"] = maximum
	}
	return out
}

func enum(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}

func pageID() map[string]any {
	return map[string]any{"type": "string", "pattern": "^sli_[A-Za-z0-9_-]+$"}
}

func nonblank() map[string]any {
	return map[string]any{"type": "string", "minLength": 1, "pattern": `\S`}
}

func array(items map[string]any, minimum, maximum int) map[string]any {
	out := map[string]any{"type": "array", "items": items}
	if minimum > 0 {
		out["minItems"] = minimum
	}
	if maximum > 0 {
		out["maxItems"] = maximum
	}
	return out
}

func exclusive(first, second string) []any {
	return []any{
		map[string]any{"required": []string{first}, "not": map[string]any{"required": []string{second}}},
		map[string]any{"required": []string{second}, "not": map[string]any{"required": []string{first}}},
	}
}

func inputSchema(name string) map[string]any {
	switch name {
	case "read_resource":
		out := object([]string{"type"}, map[string]any{"type": enum("manifest", "design", "outline", "spec", "html"), "slide_id": pageID()})
		out["if"] = map[string]any{"properties": map[string]any{"type": enum("spec", "html")}, "required": []string{"type"}}
		out["then"] = map[string]any{"required": []string{"slide_id"}}
		out["else"] = map[string]any{"not": map[string]any{"required": []string{"slide_id"}}}
		return out
	case "read_image":
		out := object(nil, map[string]any{"attachment_id": map[string]any{"type": "string", "pattern": "^att_[A-Za-z0-9_-]{1,128}$"}, "slide_id": pageID()})
		out["oneOf"] = exclusive("attachment_id", "slide_id")
		return out
	case "render_slide":
		selector := nonblank()
		selector["maxLength"] = 160
		selectors := array(selector, 1, 5)
		selectors["uniqueItems"] = true
		return object([]string{"slide_id"}, map[string]any{"slide_id": pageID(), "inspect_selectors": selectors})
	case "load_component", "load_skill":
		ids := array(map[string]any{"type": "string", "pattern": `^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`}, 1, 8)
		ids["uniqueItems"] = true
		return object([]string{"ids"}, map[string]any{"ids": ids})
	case "edit_manifest", "edit_design", "edit_spec":
		return resourceEditInput(name)
	case "edit_outline":
		return outlineInput()
	case "edit_html":
		out := object([]string{"slide_id"}, map[string]any{"slide_id": pageID(), "content": stringSchema(1, 0), "edits": textEdits()})
		out["oneOf"] = exclusive("content", "edits")
		return out
	case "git_commit", "git_commit.command":
		return object([]string{"title", "items"}, map[string]any{"title": stringSchema(1, 72), "items": array(stringSchema(1, 160), 1, 6)})
	case "run_command":
		return object([]string{"command"}, map[string]any{"command": stringSchema(1, 4096)})
	case "create_plan":
		return object([]string{"title", "content", "steps"}, map[string]any{
			"title": stringSchema(1, 0), "content": stringSchema(1, 0),
			"steps": array(object([]string{"title"}, map[string]any{"title": stringSchema(1, 0)}), 0, 0),
		})
	case "update_plan":
		return object([]string{"updates"}, map[string]any{"updates": array(object([]string{"step_id", "status"}, map[string]any{
			"step_id": stringSchema(1, 0), "status": enum("pending", "processing", "completed", "failed"),
		}), 1, 0)})
	case "request_privilege":
		return object([]string{"slide_ids", "reason"}, map[string]any{"slide_ids": array(pageID(), 1, 0), "reason": stringSchema(0, 0)})
	case "ask_user":
		options := array(object([]string{"label", "description"}, map[string]any{"label": nonblank(), "description": nonblank()}), 0, 3)
		return object([]string{"questions"}, map[string]any{"questions": array(object([]string{"question", "reason"}, map[string]any{
			"question": nonblank(), "reason": nonblank(), "options": options, "allow_custom": map[string]any{"type": "boolean"},
		}), 1, 0)})
	case "review_task":
		return object([]string{"demand"}, map[string]any{"demand": nonblank()})
	case "finish_task":
		return object([]string{"message"}, map[string]any{"message": stringSchema(0, 0), "suggested_next_inputs": array(stringSchema(0, 80), 0, 3)})
	case "submit_review":
		return object([]string{"decision", "reasons"}, map[string]any{"decision": enum("approve", "revise", "refuse"), "reasons": array(stringSchema(1, 0), 1, 0)})
	case "rename_thread":
		return object([]string{"action"}, map[string]any{"action": enum("rename", "keep"), "title": stringSchema(1, 60)})
	case "compact_context":
		return textInput(0)
	case "polish_instruction":
		return textInput(MaxPolishRunes)
	case "handoff_thread":
		return textInput(MaxHandoffRunes)
	default:
		panic("missing tool input contract: " + name)
	}
}

func textInput(maximum int) map[string]any {
	return object([]string{"title", "content"}, map[string]any{"title": stringSchema(1, MaxTextTitleRunes), "content": stringSchema(1, maximum)})
}

func textEdits() map[string]any {
	return array(object([]string{"old_text", "new_text"}, map[string]any{"old_text": stringSchema(1, 0), "new_text": stringSchema(0, 0)}), 1, 0)
}

func resourceEditInput(name string) map[string]any {
	resource := map[string]string{"edit_manifest": pptschema.ManifestName, "edit_design": pptschema.DesignName, "edit_spec": pptschema.SlideSpecName}[name]
	base := prompt.CopyToolSchema(pptschema.AuthoringSchema(resource))
	properties := base["properties"].(map[string]any)
	var required []string
	if name == "edit_spec" {
		for _, key := range []string{"purpose", "content_type", "layout"} {
			properties[key] = map[string]any{"anyOf": []any{properties[key], map[string]any{"type": "null"}}}
		}
		properties["slide_id"] = pageID()
		required = []string{"slide_id"}
	}
	out := object(required, properties)
	out["minProperties"] = len(required) + 1
	return out
}

// New nodes have no id. Runtime owns identity assignment and saved-source edits.
func outlineInput() map[string]any {
	slide := object([]string{"title"}, map[string]any{"title": stringSchema(1, 160)})
	slides := array(map[string]any{"$ref": "#/$defs/slide"}, 0, 200)
	subsection := object([]string{"title", "purpose", "slides"}, map[string]any{"title": stringSchema(1, 160), "purpose": stringSchema(1, 400), "slides": slides})
	section := object([]string{"title", "purpose", "slides", "subsections"}, map[string]any{
		"title": stringSchema(1, 160), "purpose": stringSchema(1, 400), "slides": slides,
		"subsections": array(map[string]any{"$ref": "#/$defs/subsection"}, 0, 64),
	})
	section["anyOf"] = []any{
		map[string]any{"properties": map[string]any{"slides": map[string]any{"maxItems": 0}}},
		map[string]any{"properties": map[string]any{"subsections": map[string]any{"maxItems": 0}}},
	}
	out := object(nil, map[string]any{
		"init":  object([]string{"sections"}, map[string]any{"sections": array(map[string]any{"$ref": "#/$defs/section"}, 0, 32)}),
		"edits": textEdits(),
	})
	out["oneOf"] = exclusive("init", "edits")
	out["$defs"] = map[string]any{"slide": slide, "subsection": subsection, "section": section}
	return out
}
