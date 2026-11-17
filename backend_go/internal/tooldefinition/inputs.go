package tooldefinition

import pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"

const MaxTextTitleRunes = 48

func object(required []string, properties map[string]any) map[string]any {
	for _, property := range properties {
		field := property.(map[string]any)
		if _, exists := field["description"]; !exists {
			field["description"] = ""
		}
	}
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
		out := object([]string{"type"}, map[string]any{"type": enum("meta", "constraints", "outline", "spec", "html"), "slide_id": pageID()})
		out["if"] = map[string]any{"properties": map[string]any{"type": enum("spec", "html")}, "required": []string{"type"}}
		out["then"] = map[string]any{"required": []string{"slide_id"}}
		out["else"] = map[string]any{"not": map[string]any{"required": []string{"slide_id"}}}
		return out
	case "read_image":
		out := object(nil, map[string]any{"attachment_id": map[string]any{"type": "string", "pattern": "^att_[A-Za-z0-9_-]{1,128}$"}, "slide_id": pageID()})
		out["oneOf"] = exclusive("attachment_id", "slide_id")
		return out
	case "render_slide":
		return object([]string{"slide_id", "assess_quality"}, map[string]any{"slide_id": pageID(), "assess_quality": map[string]any{"type": "boolean"}})
	case "load_component":
		return object([]string{"component_id"}, map[string]any{"component_id": map[string]any{"type": "string", "pattern": `^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`}})
	case "load_skill":
		return object([]string{"skill_id"}, map[string]any{"skill_id": map[string]any{"type": "string", "pattern": `^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`}})
	case "edit_meta", "edit_spec":
		return resourceEditInput(name)
	case "edit_constraints":
		return constraintsEditInput()
	case "edit_outline":
		return outlineInput()
	case "edit_html":
		out := object([]string{"slide_id"}, map[string]any{"slide_id": pageID(), "content": stringSchema(1, 0), "edits": textEdits()})
		out["oneOf"] = exclusive("content", "edits")
		return out
	case "git_commit":
		return object([]string{"title", "items"}, map[string]any{"title": stringSchema(1, 72), "items": array(stringSchema(1, 160), 1, 6)})
	case "run_command":
		return object([]string{"command"}, map[string]any{
			"command": nonblank(),
		})
	case "create_plan":
		return object([]string{"content", "steps"}, map[string]any{"content": nonblank(), "steps": array(nonblank(), 1, 0)})
	case "update_plan":
		steps := array(object([]string{"title", "status"}, map[string]any{
			"title": nonblank(), "status": enum("pending", "processing", "completed", "failed"),
		}), 1, 0)
		steps["type"] = []string{"array", "null"}
		return object([]string{"steps"}, map[string]any{"steps": steps})
	case "request_privilege":
		return object([]string{"slide_ids", "reason"}, map[string]any{"slide_ids": array(pageID(), 1, 0), "reason": nonblank()})
	case "ask_user":
		options := array(object([]string{"label", "description"}, map[string]any{"label": nonblank(), "description": nonblank()}), 0, 3)
		return object([]string{"questions"}, map[string]any{"questions": array(object([]string{"question", "reason"}, map[string]any{
			"question": nonblank(), "reason": nonblank(), "options": options,
		}), 1, 0)})
	case "review_ppt":
		return object([]string{"demand"}, map[string]any{"demand": nonblank()})
	case "suggest_request":
		return object([]string{"requests"}, map[string]any{"requests": array(stringSchema(1, 80), 0, 3)})
	case "rename_thread":
		out := object([]string{"action"}, map[string]any{"action": enum("rename", "keep"), "title": stringSchema(1, 60)})
		out["if"] = map[string]any{"properties": map[string]any{"action": enum("rename")}, "required": []string{"action"}}
		out["then"] = map[string]any{"required": []string{"title"}}
		out["else"] = map[string]any{"not": map[string]any{"required": []string{"title"}}}
		return out
	case "compact_context", "polish_instruction", "handoff_thread":
		return textInput()
	default:
		panic("missing tool input contract: " + name)
	}
}

func textInput() map[string]any {
	return object([]string{"title", "content"}, map[string]any{"title": stringSchema(1, MaxTextTitleRunes), "content": stringSchema(1, 0)})
}

func textEdits() map[string]any {
	return array(object([]string{"old_text", "new_text"}, map[string]any{"old_text": stringSchema(1, 0), "new_text": stringSchema(0, 0)}), 1, 0)
}

func resourceEditInput(name string) map[string]any {
	resource := map[string]string{"edit_meta": pptschema.MetaName, "edit_spec": pptschema.SlideSpecName}[name]
	base := pptschema.AuthoringSchema(resource)
	clearSchemaDescriptions(base)
	properties := base["properties"].(map[string]any)
	var required []string
	if name == "edit_spec" {
		properties["slide_id"] = pageID()
		required = []string{"slide_id"}
	}
	out := object(required, properties)
	out["minProperties"] = len(required) + 1
	return out
}

func constraintsEditInput() map[string]any {
	base := pptschema.AuthoringSchema(pptschema.ConstraintsName)
	clearSchemaDescriptions(base)
	domains := base["properties"].(map[string]any)
	content := domains["content"].(map[string]any)["properties"].(map[string]any)
	visual := domains["visual"].(map[string]any)["properties"].(map[string]any)
	out := object([]string{"type"}, map[string]any{
		"type":         enum("content", "visual"),
		"requirements": visual["requirements"], "prohibitions": visual["prohibitions"],
	})
	out["minProperties"] = 2
	out["if"] = map[string]any{"properties": map[string]any{"type": enum("content")}, "required": []string{"type"}}
	out["then"] = map[string]any{"properties": content}
	return out
}

func outlineInput() map[string]any {
	out := object(nil, map[string]any{
		"content": stringSchema(1, 0),
		"edits":   textEdits(),
	})
	out["oneOf"] = exclusive("content", "edits")
	return out
}
