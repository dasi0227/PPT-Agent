package tooldefinition

import pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"

const (
	MaxTextTitleRunes = 48
	MaxPolishRunes    = 8000
	MaxHandoffRunes   = 40000
)

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
		out := object([]string{"type"}, map[string]any{"type": enum("manifest", "design", "outline", "spec", "html"), "slide_id": pageID()})
		properties := out["properties"].(map[string]any)
		properties["type"].(map[string]any)["description"] = "`manifest` for global content requirements; `design` for global visual requirements; `outline` for page structure; `spec` for semantic content plan; `html` for exact source."
		properties["slide_id"].(map[string]any)["description"] = "Stable unique page ID from the current outline."
		out["if"] = map[string]any{"properties": map[string]any{"type": enum("spec", "html")}, "required": []string{"type"}}
		out["then"] = map[string]any{"required": []string{"slide_id"}}
		out["else"] = map[string]any{"not": map[string]any{"required": []string{"slide_id"}}}
		return out
	case "read_image":
		out := object(nil, map[string]any{"attachment_id": map[string]any{"type": "string", "pattern": "^att_[A-Za-z0-9_-]{1,128}$"}, "slide_id": pageID()})
		properties := out["properties"].(map[string]any)
		properties["attachment_id"].(map[string]any)["description"] = "Uploaded unique image ID from attachment context, rather than a file path or URL."
		properties["slide_id"].(map[string]any)["description"] = "Stable unique page ID from the current outline, rather than a file path or URL."
		out["oneOf"] = exclusive("attachment_id", "slide_id")
		return out
	case "render_slide":
		out := object([]string{"slide_id", "need_preview"}, map[string]any{"slide_id": pageID(), "need_preview": map[string]any{"type": "boolean"}})
		properties := out["properties"].(map[string]any)
		properties["slide_id"].(map[string]any)["description"] = "Stable unique page ID from the current outline."
		properties["need_preview"].(map[string]any)["description"] = "`true` requests an additional page quality assessment; `false` only returns the screenshot and diagnostics."
		return out
	case "load_component":
		return object([]string{"component_id"}, map[string]any{"component_id": map[string]any{"type": "string", "pattern": `^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`, "description": "The component's unique ID from `available_components`, rather than its name or file path."}})
	case "load_skill":
		return object([]string{"skill_id"}, map[string]any{"skill_id": map[string]any{"type": "string", "pattern": `^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`, "description": "The skill's unique ID from `available_skills`, rather than its name or file path."}})
	case "edit_manifest", "edit_design", "edit_spec":
		return resourceEditInput(name)
	case "edit_outline":
		return outlineInput()
	case "edit_html":
		out := object([]string{"slide_id"}, map[string]any{"slide_id": pageID(), "content": stringSchema(1, 0), "edits": textEdits()})
		properties := out["properties"].(map[string]any)
		properties["slide_id"].(map[string]any)["description"] = "Stable unique page ID from the current outline."
		properties["content"].(map[string]any)["description"] = "The complete HTML document to create or replace, without Markdown fences."
		edits := properties["edits"].(map[string]any)
		edits["description"] = "The ordered list of exact text replacements to apply to the current saved HTML."
		editProperties := edits["items"].(map[string]any)["properties"].(map[string]any)
		editProperties["old_text"].(map[string]any)["description"] = "The exact, non-empty source text to replace, including whitespace."
		editProperties["new_text"].(map[string]any)["description"] = "The literal replacement source; use an empty string to delete the matched text."
		out["oneOf"] = exclusive("content", "edits")
		return out
	case "git_commit":
		out := object([]string{"title", "items"}, map[string]any{"title": stringSchema(1, 72), "items": array(stringSchema(1, 160), 1, 6)})
		properties := out["properties"].(map[string]any)
		properties["title"].(map[string]any)["description"] = "Concise single-line plain-text title that summarizes the primary change, in the format `<feat|fix|refactor|perf|chore|docs>: <primary change>`, without Markdown, HTML, surrounding quotation marks, status prefixes, or a trailing period."
		properties["items"].(map[string]any)["description"] = "Distinct body entries, each summarizing one coherent change in a single sentence."
		return out
	case "run_command":
		return object([]string{"command"}, map[string]any{"command": stringSchema(1, 4096)})
	case "create_plan":
		out := object([]string{"title", "content", "steps"}, map[string]any{
			"title": nonblank(), "content": nonblank(),
			"steps": array(nonblank(), 1, 0),
		})
		properties := out["properties"].(map[string]any)
		properties["title"].(map[string]any)["description"] = "User-facing title of the proposed work."
		properties["content"].(map[string]any)["description"] = "Complete Markdown plan organized into understanding of the request, implementation approach, detailed steps, relevant considerations, and expected outcomes. Keep the detailed steps consistent with `steps`."
		properties["steps"].(map[string]any)["description"] = "Ordered array of strings for the complete proposed plan. Each string is a single concise sentence that precisely summarizes the step's action or deliverable recognizable in plan progress."
		return out
	case "update_plan":
		out := object([]string{"updates"}, map[string]any{"updates": array(object([]string{"step_id", "status"}, map[string]any{
			"step_id": nonblank(), "status": enum("pending", "processing", "completed", "failed"),
		}), 1, 0)})
		updates := out["properties"].(map[string]any)["updates"].(map[string]any)
		updates["description"] = "Status changes for existing approved steps."
		updateProperties := updates["items"].(map[string]any)["properties"].(map[string]any)
		updateProperties["step_id"].(map[string]any)["description"] = "Exact unique step ID from the current plan context."
		updateProperties["status"].(map[string]any)["description"] = "`pending` for work awaiting execution; `processing` for work in progress; `completed` for finished work that achieved its expected result; `failed` for work whose execution failed."
		return out
	case "request_privilege":
		out := object([]string{"slide_ids", "reason"}, map[string]any{"slide_ids": array(pageID(), 1, 0), "reason": nonblank()})
		properties := out["properties"].(map[string]any)
		properties["reason"].(map[string]any)["description"] = "User-facing explanation of why the authorized slide scope needs to be expanded."
		properties["slide_ids"].(map[string]any)["description"] = "IDs of the slides to add to the current authorized scope."
		return out
	case "ask_user":
		options := array(object([]string{"label", "description"}, map[string]any{"label": nonblank(), "description": nonblank()}), 0, 3)
		out := object([]string{"questions"}, map[string]any{"questions": array(object([]string{"question", "reason"}, map[string]any{
			"question": nonblank(), "reason": nonblank(), "options": options,
		}), 1, 0)})
		questions := out["properties"].(map[string]any)["questions"].(map[string]any)
		questions["description"] = "Ordered questions to present together, each addressing one fact or decision."
		questionProperties := questions["items"].(map[string]any)["properties"].(map[string]any)
		questionProperties["question"].(map[string]any)["description"] = "Complete user-visible interrogative sentence, ending with question mark."
		questionProperties["reason"].(map[string]any)["description"] = "Brief user-visible explanation of why the answer is needed and what subsequent work it affects."
		questionProperties["options"].(map[string]any)["description"] = "One to three preset answers for a single-choice question; custom text is always available. Omit this field or pass `[]` to show a text input instead."
		optionProperties := options["items"].(map[string]any)["properties"].(map[string]any)
		optionProperties["label"].(map[string]any)["description"] = "Concise and distinct option text. If you strongly recommend this option, append `(Recommended)` to its end."
		optionProperties["description"].(map[string]any)["description"] = "Concrete meaning, effect or tradeoff of this option."
		return out
	case "review_ppt":
		out := object([]string{"demand"}, map[string]any{"demand": nonblank()})
		out["properties"].(map[string]any)["demand"].(map[string]any)["description"] = "Self-contained instructions for the reviewer, including the review objective, key areas to focus on, acceptance criteria, relevant user requirements and decisions, and the feedback you need."
		return out
	case "finish_task":
		out := object([]string{"message"}, map[string]any{"message": nonblank(), "suggested_next_inputs": array(stringSchema(0, 80), 0, 3)})
		properties := out["properties"].(map[string]any)
		properties["message"].(map[string]any)["description"] = "Complete final user-facing answer to the current request, following the requirements in `core.output`."
		properties["suggested_next_inputs"].(map[string]any)["description"] = "Optional list of up to three requests the user is most likely to make immediately after this task, inferred from conversation history, the current project state, and the work just completed, ordered by usefulness. Write each as one self-contained, ready-to-send plain-text sentence in the user's language, closely matching their wording and style in previous messages."
		return out
	case "rename_thread":
		out := object([]string{"action"}, map[string]any{"action": enum("rename", "keep"), "title": stringSchema(1, 60)})
		properties := out["properties"].(map[string]any)
		properties["title"].(map[string]any)["description"] = "Concise single-line plain-text title that identifies the conversation's main topic or current stage of work, without Markdown, HTML, surrounding quotation marks, status prefixes, or a trailing period."
		properties["action"].(map[string]any)["description"] = "`rename` for a missing title or one that no longer represents the recent main topic; `keep` for a title that still fits or when the available information is insufficient to choose a better one."
		out["if"] = map[string]any{"properties": map[string]any{"action": enum("rename")}, "required": []string{"action"}}
		out["then"] = map[string]any{"required": []string{"title"}}
		out["else"] = map[string]any{"not": map[string]any{"required": []string{"title"}}}
		return out
	case "compact_context":
		out := textInput(0)
		properties := out["properties"].(map[string]any)
		properties["title"].(map[string]any)["description"] = "Concise single-line plain-text title that identifies the task, stage, or key decision captured in the summary, without Markdown, HTML, surrounding quotation marks, status prefixes, or a trailing period."
		properties["content"].(map[string]any)["description"] = "Complete Markdown working summary for continuing the same task."
		return out
	case "polish_instruction":
		out := textInput(MaxPolishRunes)
		properties := out["properties"].(map[string]any)
		properties["title"].(map[string]any)["description"] = "Concise single-line plain-text title that describes the wording improvement or indicates that the draft is unchanged, without Markdown, HTML, surrounding quotation marks, status prefixes, or a trailing period."
		properties["content"].(map[string]any)["description"] = "Complete revised draft ready to send, or the original draft if no changes are needed."
		return out
	case "handoff_thread":
		out := textInput(MaxHandoffRunes)
		properties := out["properties"].(map[string]any)
		properties["title"].(map[string]any)["description"] = "Concise single-line plain-text title that identifies the work or stage being handed over, without Markdown, HTML, surrounding quotation marks, status prefixes, or a trailing period."
		properties["content"].(map[string]any)["description"] = "Complete standalone Markdown handoff brief, ready to send as the first message to the receiving Agent."
		return out
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
	base := pptschema.AuthoringSchema(resource)
	clearSchemaDescriptions(base)
	properties := base["properties"].(map[string]any)
	if name == "edit_manifest" {
		properties["title"].(map[string]any)["description"] = "A concise title for the entire presentation that captures its main theme, scope, and content."
		properties["audience"].(map[string]any)["description"] = "A concise description of the presentation's target audience, guiding content depth and terminology complexity."
		properties["goal"].(map[string]any)["description"] = "A concise statement of the presentation's purpose, focusing on what the audience should understand or be able to do."
		properties["language"].(map[string]any)["description"] = "The language used in the presentation, specified as a language tag such as `en-US` or `zh-CN`."
		properties["pages"].(map[string]any)["description"] = "The desired number of slides in the presentation, which may be a range or an approximation."
		properties["requirements"].(map[string]any)["description"] = "Each item specifies a concrete content requirement tailored to this presentation and shared across slides."
		properties["prohibitions"].(map[string]any)["description"] = "Each item specifies a concrete content restriction tailored to this presentation and shared across slides."
	} else if name == "edit_design" {
		properties["requirements"].(map[string]any)["description"] = "Each item specifies a concrete visual requirement tailored to this presentation and shared across slides."
		properties["prohibitions"].(map[string]any)["description"] = "Each item specifies a concrete visual restriction tailored to this presentation and shared across slides."
	}
	var required []string
	if name == "edit_spec" {
		for _, key := range []string{"core", "purpose", "approach", "layout"} {
			properties[key] = map[string]any{"anyOf": []any{properties[key], map[string]any{"type": "null"}}}
		}
		properties["slide_id"] = pageID()
		properties["slide_id"].(map[string]any)["description"] = "Stable unique page ID from the current outline."
		properties["core"].(map[string]any)["description"] = "A concise statement of the main takeaway the audience should retain from this page, rather than its title, a content inventory or layout instructions."
		properties["layout"].(map[string]any)["description"] = "A concise statement of how content is spatially arranged to improve visual appeal and readability, rather than specific HTML or CSS implementation details."
		properties["purpose"].(map[string]any)["description"] = "`cover` for topic and identity; `introduction` for background, problem, goals or structure; `transition` for a chapter change; `content` for body material; `conclusion` for summary, conclusions or actions; `other` for a clearly known role outside these categories."
		properties["approach"].(map[string]any)["description"] = "`explanation` for concepts or facts; `comparison` for differences or tradeoffs; `example` for concrete cases; `guidance` for methods or recommendations; `other` for a clearly known mode outside these categories."
		elements := properties["elements"].(map[string]any)
		elements["description"] = "The complete ordered list of semantic content units for this page; order follows how the audience should read them."
		elementProperties := elements["items"].(map[string]any)["properties"].(map[string]any)
		elementProperties["type"].(map[string]any)["description"] = "`text` for prose; `list` for items; `metric` for a key figure; `quote` for a quotation; `table` for tabular values; `chart` for data visualization; `diagram` for relationships; `code` for a code sample; `asset` for visual media."
		elementProperties["intent"].(map[string]any)["description"] = "A concise statement of the concrete message, facts, evidence or relationship this element should convey and why it belongs on this page."
		required = []string{"slide_id"}
	}
	out := object(required, properties)
	out["minProperties"] = len(required) + 1
	return out
}

func outlineInput() map[string]any {
	out := object(nil, map[string]any{
		"content": stringSchema(1, 0),
		"edits":   textEdits(),
	})
	properties := out["properties"].(map[string]any)
	properties["content"].(map[string]any)["description"] = "Complete outline JSON source to create or fully replace the outline, without Markdown fences. Preserve existing IDs and use `id: \"new\"` for each new node."
	edits := properties["edits"].(map[string]any)
	edits["description"] = "Ordered exact text replacements in the current saved outline JSON source. Preserve existing IDs and use `id: \"new\"` for each added node."
	editProperties := edits["items"].(map[string]any)["properties"].(map[string]any)
	editProperties["old_text"].(map[string]any)["description"] = "Exact, non-empty source text to replace, including whitespace and enough surrounding context to identify one occurrence."
	editProperties["new_text"].(map[string]any)["description"] = "Literal replacement source; use an empty string to delete the matched text."
	out["oneOf"] = exclusive("content", "edits")
	return out
}
