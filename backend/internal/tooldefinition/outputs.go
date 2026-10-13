package tooldefinition

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"
)

func constant(value string) map[string]any {
	return map[string]any{"type": "string", "const": value}
}

// OutputContract constructs the structural normal-result contract for an entry.
func OutputContract(name string) map[string]any {
	var out map[string]any
	switch name {
	case "read_resource":
		out = object([]string{"content"}, map[string]any{"content": stringSchema(0, 0)})
	case "edit_manifest", "edit_design", "edit_spec":
		resource := map[string]string{"edit_manifest": "manifest", "edit_design": "design", "edit_spec": "spec"}[name]
		out = object([]string{"content", "changed_fields", "changed"}, map[string]any{
			"content": resourceContent(resource), "changed_fields": array(stringSchema(0, 0), 0, 0), "changed": map[string]any{"type": "boolean"},
		})
	case "edit_outline":
		out = object([]string{"content", "changed"}, map[string]any{"content": stringSchema(0, 0), "changed": map[string]any{"type": "boolean"}})
	case "edit_html":
		out = object([]string{"summary", "changed"}, map[string]any{"summary": stringSchema(0, 0), "changed": map[string]any{"type": "boolean"}, "content_precheck": contentPrecheck()})
	case "load_component", "load_skill":
		out = object([]string{"status"}, map[string]any{"status": enum("success", "already_available")})
	case "run_command":
		out = object([]string{"stdout", "stderr", "exit_code"}, map[string]any{
			"stdout": stringSchema(0, 0), "stderr": stringSchema(0, 0), "exit_code": map[string]any{"type": "integer"}, "content_precheck": contentPrecheck(),
		})
	case "git_commit":
		out = object([]string{"summary"}, map[string]any{"summary": stringSchema(0, 0), "hash": stringSchema(0, 0), "branch": stringSchema(0, 0)})
	case "create_plan", "request_privilege":
		decisions := enum("approve", "refuse")
		if name == "request_privilege" {
			decisions = enum("approve", "revise", "refuse")
		}
		out = object([]string{"decision", "summary"}, map[string]any{"decision": decisions, "summary": stringSchema(0, 0), "feedback": stringSchema(0, 0)})
	case "update_plan":
		out = object([]string{"summary"}, map[string]any{"summary": stringSchema(0, 0)})
	case "ask_user":
		out = object([]string{"answers"}, map[string]any{"answers": array(object([]string{"question", "answer"}, map[string]any{"question": stringSchema(0, 0), "answer": stringSchema(0, 0)}), 0, 0)})
	case "review_task":
		out = object([]string{"decision", "reasons"}, map[string]any{"decision": enum("approve", "revise", "refuse"), "reasons": array(stringSchema(0, 0), 1, 0)})
	case "render_slide":
		out = renderOutput()
	case "read_image", "finish_task", "submit_review", "rename_thread", "compact_context", "polish_instruction", "handoff_thread", "git_commit.command":
		return nil
	default:
		panic("missing tool output contract: " + name)
	}
	switch name {
	case "run_command", "edit_manifest", "edit_design", "edit_outline":
		out = map[string]any{"oneOf": []any{out, object([]string{"decision", "summary"}, map[string]any{
			"decision": constant("refuse"), "summary": stringSchema(0, 0), "feedback": stringSchema(0, 0),
		})}}
	}
	return out
}

func resourceContent(resource string) map[string]any {
	name := map[string]string{"manifest": pptschema.ManifestName, "design": pptschema.DesignName, "spec": pptschema.SlideSpecName}[resource]
	return prompt.CopyToolSchema(pptschema.AuthoringSchema(name))
}

func contentPrecheck() map[string]any {
	properties := map[string]any{}
	var required []string
	for _, rubric := range model.ContentPrecheckRubrics() {
		properties[rubric.Dimension] = map[string]any{"type": "number", "minimum": 0, "maximum": len(rubric.Criteria) - 1}
		required = append(required, rubric.Dimension)
	}
	return map[string]any{"oneOf": []any{
		object([]string{"status", "scores"}, map[string]any{"status": constant("completed"), "scores": object(required, properties)}),
		object([]string{"status", "reason"}, map[string]any{"status": enum("unavailable", "skipped", "stale"), "reason": stringSchema(0, 0)}),
	}}
}

func renderOutput() map[string]any {
	rect := map[string]any{}
	for _, edge := range []string{"left", "top", "right", "bottom"} {
		rect[edge] = map[string]any{"type": "integer"}
	}
	diagnostics := object([]string{"overflow", "out_of_bounds", "console_errors", "failed_resources"}, map[string]any{
		"overflow":       object([]string{"horizontal", "vertical"}, map[string]any{"horizontal": stringSchema(0, 0), "vertical": stringSchema(0, 0)}),
		"out_of_bounds":  array(object([]string{"tag", "rect"}, map[string]any{"tag": stringSchema(0, 0), "id": stringSchema(0, 0), "class": stringSchema(0, 0), "rect": object([]string{"left", "top", "right", "bottom"}, rect)}), 0, 0),
		"console_errors": array(stringSchema(0, 0), 0, 0), "failed_resources": array(stringSchema(0, 0), 0, 0),
	})
	return object([]string{"slide_id", "diagnostics"}, map[string]any{"slide_id": stringSchema(0, 0), "diagnostics": diagnostics})
}
