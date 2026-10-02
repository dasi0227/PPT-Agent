package workflow

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	pptschema "github.com/dasi0227/PPT-Agent/backend/schemas"
)

// These contracts describe model observations, not persisted ToolResult or UI data.
// x-content-kind and x-error-schema are transport annotations consumed as documentation
// by the LLM adapter; neither adds fields to actual results or enables validation.
func toolOutputSchema(name string) map[string]any {
	var out map[string]any
	switch name {
	case "read_resource":
		variants := []any{}
		for _, resource := range []string{"manifest", "design", "spec", "outline", "html"} {
			content := resourceOutputContent(resource)
			props := map[string]any{"resource": outputConst(resource, "Resource type actually read; determines the content format."), "content": content}
			required := []string{"resource", "content"}
			if resource == "spec" || resource == "html" {
				props["slide_id"] = outputString("Stable ID of the page read.")
				required = append(required, "slide_id")
			}
			variants = append(variants, outputObject("Result for "+resource+".", required, props))
		}
		out = map[string]any{"description": "The complete current resource, without version hashes or a data wrapper.", "oneOf": variants}
	case "edit_manifest", "edit_design", "edit_spec":
		resource := map[string]string{"edit_manifest": "manifest", "edit_design": "design", "edit_spec": "spec"}[name]
		out = outputObject("Saved resource; omitted input fields are included with their retained values.", []string{"content", "changed_fields"}, map[string]any{
			"content":        resourceOutputContent(resource),
			"changed_fields": outputArray("Top-level fields whose stored values changed; empty for a no-op.", outputString("Changed field name in content.")),
		})
	case "edit_outline":
		out = outputObject("Saved outline including generated stable IDs.", []string{"content"}, map[string]any{"content": resourceOutputContent("outline")})
	case "edit_html":
		out = outputSummary("HTML save acknowledgement; no source is echoed. Saving does not verify appearance; call render_slide.")
	case "load_component", "load_skill":
		key, body := "components", "Complete component HTML source for reference and adaptation."
		if name == "load_skill" {
			key, body = "skills", "Complete skill instructions made available to this run."
		}
		out = outputObject("Requested repository bodies, partitioned by visibility in the current model context.", []string{key, "already_available"}, map[string]any{
			key: outputArray("Entries whose full bodies are newly supplied; empty if all are already visible.", outputObject("One requested repository entry.", []string{"id", "content"}, map[string]any{
				"id": outputString("Stable catalog ID of this entry."), "content": outputString(body),
			})),
			"already_available": outputArray("IDs whose full bodies are already in the current model context; catalog listings alone do not count.", outputString("Requested entry ID whose body is not repeated.")),
		})
	case "run_command":
		out = outputObject("Captured command result. Output limits terminate execution; failures may retain partial output. No full-output file or continuation handle is provided.", []string{"stdout", "stderr", "exit_code"}, commandOutputProperties())
	case "git_commit":
		out = outputObject("Local commit outcome returned in the agent loop; no remote push occurs. In /commit, the host publishes the command result and ends execution without sending a tool reply to the model.", []string{"summary"}, map[string]any{
			"summary": outputString("Commit acknowledgement, or a message that there were no changes to commit."),
			"hash":    outputString("Git commit identifier, present only when a commit was created."),
			"branch":  outputString("Branch where the commit was created; omitted with hash when there were no changes."),
		})
	case "create_plan", "request_privilege":
		decision, summary := "approve freezes the approved plan and permits execution; revise requires revising the draft and calling create_plan again; refuse forbids executing or automatically resubmitting that plan.", "Approval/refusal notice, or the user's plan revision feedback verbatim; if no feedback was supplied, states that explicitly."
		if name == "request_privilege" {
			decision = "approve grants the requested pages (or confirms they were already authorized); revise immediately authorizes all pages, including pages created during this run, with no resubmission; refuse keeps the previous scope."
			summary = "Explanation of the effective page authorization, including when the requested pages were already authorized."
		}
		out = outputObject("User approval outcome returned through the original call after waiting, when approval is needed. Refusal and revision are business decisions, not execution errors.", []string{"decision", "summary"}, map[string]any{
			"decision": outputEnum(decision, "approve", "revise", "refuse"), "summary": outputString(summary),
		})
	case "update_plan":
		out = outputSummary("Acknowledgement that the requested step statuses were saved; does not return or replace the plan.")
	case "ask_user":
		out = outputObject("User answers after this call resumes. Cancellation or interaction failure does not fabricate answers.", []string{"answers"}, map[string]any{
			"answers": outputArray("One item per original question in the original order, including skipped questions.", outputObject("Answer paired with its original question.", []string{"question", "answer"}, map[string]any{
				"question": outputString("Original questions[].question text, not an internal question ID."),
				"answer":   outputString("Selected option label, or the user's verbatim custom/free-text answer. A skipped question returns this string instead of null: " + skippedQuestionAnswer + " Skipping is not explicit approval."),
			})),
		})
	case "review_task":
		out = outputObject("Independent artifact assessment; does not grant user authorization or finish the main task. Reviewer execution failure is an error, not a review decision.", []string{"decision", "reasons"}, map[string]any{
			"decision": outputEnum("approve supports delivery; revise requires verification or revision; refuse identifies defects blocking delivery.", "approve", "revise", "refuse"),
			"reasons":  outputArray("Non-empty findings for every decision, including the evidence supporting approval.", outputString("Concrete approval basis, uncertainty or revision needed, or confirmed defect blocking delivery.")),
		})
		out["properties"].(map[string]any)["reasons"].(map[string]any)["minItems"] = 1
	case "render_slide":
		out = renderOutputSchema()
		out["x-content-kind"] = "json+image"
	case "read_image":
		out = map[string]any{"x-content-kind": "image", "description": "On success, only the actual image content block is returned: uploaded image or latest valid page screenshot. There is no JSON image field, path, URL or metadata text. Missing or stale screenshots require render_slide first."}
	case "finish_task":
		out = llm.NoReplyOutput("On success, Runtime finishes this run and publishes the submitted message as the final answer; there is no tool reply. A rejected completion returns error JSON so the model can address the reported issues.")
	case "submit_review":
		return llm.SubmissionNoReplyOutput("On success, the Reviewer loop ends without a tool reply. Its decision and reasons are delivered to the main agent through review_task; this does not finish the main task. Rejected submissions receive specific error feedback and may be corrected within the runtime budget.")
	default:
		panic("missing output contract for tool: " + name)
	}
	if _, ok := out["x-content-kind"]; !ok {
		out["x-content-kind"] = "json"
	}
	out["x-error-schema"] = toolErrorOutputSchema(name)
	return out
}

func outputString(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}
func outputEnum(description string, values ...string) map[string]any {
	out := outputString(description)
	out["enum"] = values
	return out
}
func outputConst(value, description string) map[string]any {
	out := outputString(description)
	out["const"] = value
	return out
}
func outputArray(description string, items map[string]any) map[string]any {
	return map[string]any{"type": "array", "description": description, "items": items}
}
func outputObject(description string, required []string, properties map[string]any) map[string]any {
	out := objectSchema(required, properties)
	out["description"] = description
	return out
}
func outputSummary(description string) map[string]any {
	return outputObject(description, []string{"summary"}, map[string]any{"summary": outputString(description)})
}
func outputNumber(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}
func outputBool(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

// The persisted resource schemas are reused without altering input constraints.
// Output-specific annotations replace only authoring instructions that would be
// misleading when reading an already saved value.
func resourceOutputContent(resource string) map[string]any {
	if resource == "html" {
		return outputString("Exact saved HTML source text, suitable for subsequent exact text edits.")
	}
	if resource == "outline" {
		return outputString("Exact saved outline JSON source string, not a parsed object; use directly for exact edits. sections contains ordered top-level groups with id, title, purpose, slides and subsections. Subsections have id, title, purpose and slides; each page has slide_id and title. title is the visible heading of a section, subsection or page; purpose explains a section or subsection's contribution to the narrative. IDs are stable backend-assigned identities, not positions. A section uses direct slides or subsections, not both; unused arrays are empty.")
	}
	name := map[string]string{"manifest": pptschema.ManifestName, "design": pptschema.DesignName, "spec": pptschema.SlideSpecName}[resource]
	out := pptschema.AuthoringSchema(name)
	delete(out, "examples")
	out["description"] = "Complete saved " + resource + " JSON object."
	props := out["properties"].(map[string]any)
	switch resource {
	case "manifest":
		props["requirements"].(map[string]any)["description"] = "Additional required content, evidence and wording conventions; empty when none are specified."
		props["prohibitions"].(map[string]any)["description"] = "Explicit content exclusions and prohibitions; empty when none are specified."
	case "design":
		props["decorations"].(map[string]any)["description"] = "Saved positions of shared decorations. Every non-none position is unique across all configured decorations, even if text is currently missing. left-edge and right-edge mean the vertical midpoint; none hides an optional decoration. Text derives from presentation resources; appearance comes from Runtime and the theme."
	case "spec":
		props["elements"].(map[string]any)["description"] = "Ordered content elements planned for this page; an empty array means no elements are specified. Each item pairs representation and communication intent, not HTML."
	}
	return out
}

func commandOutputProperties() map[string]any {
	truncated := outputBool("Present only as true when captured output was truncated; absent otherwise. The command was terminated on exceeding the output limit.")
	truncated["const"] = true
	return map[string]any{
		"stdout":           outputString("Captured standard output; may be incomplete on failure or truncation."),
		"stderr":           outputString("Captured standard error; empty when none was captured."),
		"exit_code":        outputNumber("Command exit status. On execution failure, interpret with code/reason; a default value alone does not prove the command ran successfully."),
		"output_truncated": truncated,
	}
}

func renderOutputSchema() map[string]any {
	rect := map[string]any{}
	for _, edge := range []string{"left", "top", "right", "bottom"} {
		rect[edge] = outputNumber("Element's " + edge + " edge relative to the page iframe viewport, in CSS pixels rounded to an integer.")
	}
	return outputObject("Actual screenshot image block plus this JSON diagnostic text. The image is not a JSON field or a path. A valid screenshot may still have blocking diagnostics; fix them and render again.", []string{"slide_id", "diagnostics"}, map[string]any{
		"slide_id": outputString("Stable ID of the rendered page."),
		"diagnostics": outputObject("Observed rendering problems; these checks do not prove complete visual correctness.", []string{"overflow", "out_of_bounds", "console_errors", "failed_resources"}, map[string]any{
			"overflow": outputObject("Scrollable content size compared to canvas viewport with 1px tolerance; independent of individual element bounds.", []string{"horizontal", "vertical"}, map[string]any{
				"horizontal": outputString("Horizontal overflow verdict with content width, canvas width and excess pixels when overflowing."),
				"vertical":   outputString("Vertical overflow verdict with content height, canvas height and excess pixels when overflowing."),
			}),
			"out_of_bounds": outputArray("Up to 50 descendants extending beyond .slide-stage (document root if absent), allowing 1px tolerance; empty if none. Does not detect all clipping, ellipsis or overlap.", outputObject("One element whose bounds extend outside the stage.", []string{"tag", "rect"}, map[string]any{
				"tag": outputString("Lowercase HTML tag name."), "id": outputString("Element ID, omitted when absent."),
				"class": outputString("Element class text, when present, truncated to at most 160 characters."),
				"rect":  outputObject("Element bounding rectangle in the page iframe viewport.", []string{"left", "top", "right", "bottom"}, rect),
			})),
			"console_errors":   outputArray("Up to 50 console error messages or uncaught script errors, at most 1000 characters each; empty if none.", outputString("Captured browser error message.")),
			"failed_resources": outputArray("Up to 50 deduplicated failed requests, HTTP 400+ responses or blocked external resources; empty if none.", outputString("Failure cause or HTTP status with the resource address.")),
		}),
		"code":   outputConst(CodeRenderFailed, "Present when the screenshot exists but diagnostics contain blocking issues; this does not imply the screenshot is missing."),
		"reason": outputString("Explanation of blocking diagnostics requiring page changes and a new render; omitted when no blocking issue was found."),
	})
}

func toolErrorOutputSchema(name string) map[string]any {
	props := map[string]any{
		"code":        outputString("Machine-readable failure code; use reason and next_action to decide how to recover."),
		"category":    outputEnum("Failure class: transient may be retried; agent_repairable requires correction; user_action_required needs user intervention; conflict requires refreshing state; canceled stops the operation; terminal cannot be repaired by repeating the call.", "transient", "agent_repairable", "user_action_required", "conflict", "canceled", "terminal"),
		"reason":      outputString("Concrete reason this call failed; not a successful business result."),
		"retryable":   outputBool("Whether Runtime classifies this failure as automatically retryable; false does not prevent a corrected call after following next_action."),
		"next_action": outputString("Recovery guidance for this failure; follow it before retrying an unchanged operation."),
		"call_id":     outputString("Original tool call identifier, when bound by Runtime; not a resource ID."),
		"operation":   outputString("Operation that failed, usually tool_call."),
		"resource": outputObject("Affected resource, when identified.", []string{"type", "part"}, map[string]any{
			"type": outputString("Resource scope, such as deck or slide."), "part": outputString("Affected resource part, such as manifest, outline, design, spec or html."),
			"slide_id": outputString("Affected page's stable ID, when applicable."),
		}),
		"field":    outputString("Path to the invalid input or saved-content field; / denotes the root."),
		"expected": outputString("Expected JSON type, included when a type mismatch is diagnosed."),
		"actual":   outputString("Observed JSON type, not the field's full value."),
	}
	switch name {
	case "read_resource", "edit_manifest", "edit_design", "edit_spec", "edit_outline", "edit_html":
		props["byte_offset"] = outputNumber("One-based byte offset after the detected JSON syntax error in the source.")
		props["validation_errors_truncated"] = outputBool("Present as true when validation findings were capped at 32; further problems may remain.")
		props["validation_errors"] = outputArray("Up to 32 individual schema validation findings.", outputObject("One invalid field or constraint.", []string{"field", "reason"}, map[string]any{
			"field": outputString("Path to the invalid field in the resource JSON."), "reason": outputString("Constraint that failed at this field."),
			"expected": outputString("Expected JSON type when available."), "actual": outputString("Observed JSON type when available."),
		}))
		if name == "read_resource" || name == "edit_html" {
			props["html_checks"] = outputArray("HTML-related failure checks or repair guidance, included when the failed call is bound to HTML.", outputString("One reported HTML issue or suggested correction."))
		}
		if name == "edit_outline" || name == "edit_html" {
			props["edit_index"] = outputNumber("Zero-based index in the submitted edits array of the failed replacement; the batch was not saved.")
			props["match_count"] = outputNumber("Occurrences of old_text at that replacement step: zero means missing, more than one means ambiguous.")
		}
	case "run_command":
		for key, value := range commandOutputProperties() {
			props[key] = value
		}
	case "finish_task":
		props["issues"] = outputArray("Completion blockers that must be addressed before resubmitting finish_task.", outputObject("One unmet completion requirement.", []string{"code", "summary"}, map[string]any{
			"code": outputString("Machine-readable completion blocker."), "summary": outputString("Which requested outcome or evidence is still missing."),
			"next_action": outputString("Suggested repair for this completion issue, when provided."),
			"required_actions": outputArray("Concrete actions required to resolve this issue, when supplied.", outputObject("One required action on an identified target.", []string{"target"}, map[string]any{
				"tool": outputString("Tool to call if currently disclosed."), "op": outputString("Operation identifier when the issue specifies one."),
				"target": outputObject("Resource affected by the required action.", []string{"type", "part"}, map[string]any{
					"type": outputString("Target scope: deck, slide or file."), "part": outputString("Target resource part."),
					"slide_id": outputString("Stable page ID for a slide target."), "path": outputString("Project-relative path for a file target."),
				}),
			})),
		}))
	}
	out := outputObject("Failure reply as JSON text instead of the normal result. Optional detail fields appear only when relevant; no ok or data wrapper is returned.", []string{"code", "category", "reason", "retryable", "next_action"}, props)
	// Error details are extensible; the known business fields above are documented.
	out["additionalProperties"] = true
	return out
}
