package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

const outlineSourceContract = "The root requires a sections array. Sections contain id, title, slides and subsections; subsections contain id, title and slides; slides contain only id and title. Every node requires id: use \"new\" for each new node and preserve existing IDs when moving or editing. Runtime replaces each \"new\" with a distinct real ID. Only new containers may omit child arrays, which default to []; existing containers require them, and null is invalid. Do not invent or duplicate existing IDs or change their node types. Pass the returned page id as slide_id when calling page tools. Use either direct slides or subsections per section, never both."

func outlineMissingFailure(resource Resource) ToolResult {
	return failedToolResult(model.CodeOutlineNotExist, "The outline source .outline.json does not exist; "+resource.Key()+" requires it to locate valid page IDs.")
}

func resourceReadAction(resource Resource) string {
	args := map[string]any{"type": resource.Part}
	if resource.SlideID != "" {
		args["slide_id"] = resource.SlideID
	}
	raw, _ := json.Marshal(args)
	return "Call read_resource with " + string(raw) + " to obtain the current saved content, then regenerate the edit from that content. Do not blindly retry unchanged parameters."
}

func resourceConflictFailure(resource Resource, unseen bool) ToolResult {
	reason := "The " + resource.Key() + " resource has changed since it was last supplied to the model."
	if unseen {
		reason = "The current " + resource.Key() + " resource has not been read or supplied to the model."
	}
	return detailedToolFailure(model.CodeContentConflict, reason, map[string]any{"measure": resourceReadAction(resource)})
}

func resourceApprovalConflictFailure(resource Resource) ToolResult {
	return detailedToolFailure(model.CodeContentConflict, "The approval or source baseline for "+resource.Key()+" changed; the previous approval cannot authorize this edit.", map[string]any{
		"measure": resourceReadAction(resource) + " Submit the revised edit for a fresh approval when required; do not reuse the stale approval.",
	})
}

func resourceReadFailure(err error, resource Resource) ToolResult {
	if errors.Is(err, spec.ErrInvalid) {
		return savedResourceInvalid(err, resource)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return readFailure(err)
	}
	if resource.Part == "outline" {
		return outlineMissingFailure(resource)
	}
	action := "The project resource is missing. Report the missing resource; do not invent a replacement or repeat an unchanged read."
	switch resource.Part {
	case "html":
		action = "If this slide_id matches a page's id in the current outline, create its HTML with edit_html when disclosed, then render_slide. Otherwise read the outline and pass an existing page's id as slide_id. Do not patch, render or read missing HTML repeatedly."
	case "spec":
		action = "If this slide_id matches a page's id in the current outline, use edit_spec when disclosed to create its spec with elements; include approach when purpose is content. Otherwise read the outline and pass an existing page's id as slide_id. Do not repeat an unchanged read."
	}
	return detailedToolFailure(model.CodeNotFound, resource.Key()+" content was not found", map[string]any{"measure": action})
}

// Source readers and field-edit tools share saved-source validation diagnostics.
func resourceReadToolFailure(err error, resource Resource) ToolResult {
	var source *pptmutation.ResourceSourceError
	if errors.As(err, &source) {
		if source.Resource == "outline" && errors.Is(source.Err, fs.ErrNotExist) {
			return outlineMissingFailure(resource)
		}
		return resourceReadToolFailure(source.Err, Resource{Type: "deck", Part: source.Resource})
	}
	if errors.Is(err, pptmutation.ErrOutlineNotExist) {
		return outlineMissingFailure(resource)
	}
	if errors.Is(err, spec.ErrInvalid) {
		return savedResourceInvalid(err, resource)
	}
	if errors.Is(err, pptmutation.ErrSlideNotFound) {
		return detailedToolFailure(model.CodeNotFound, "Page "+resource.SlideID+" is absent from the current outline or has been deleted.", map[string]any{"measure": "Read the existing outline with read_resource(type: outline) and verify the page ID. Create a missing page with edit_outline only when disclosed, authorized and needed, then use its returned ID. Do not reinitialize the existing outline or repeatedly read an absent page."})
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return readFailure(err)
	}
	action := "Report the missing resource; do not repeat an unchanged read."
	switch resource.Part {
	case "outline":
		action = "The saved outline is missing. Create it with edit_outline(content: <complete JSON source string>) when disclosed and creation is needed, using id: \"new\" for every node, then use its returned page IDs. Do not repeat reads or invent page IDs."
	case "manifest", "design":
		action = "Create the missing " + resource.Part + " with edit_" + resource.Part + " when disclosed and needed. Do not repeat reads of missing content."
	case "spec", "html":
		action = "The page exists in the current outline, but its " + resource.Part + " has not been created. Create it with edit_" + resource.Part + " when disclosed before reading it again. Do not repeat an unchanged read."
	}
	return detailedToolFailure(model.CodeNotFound, resource.Key()+" resource does not exist.", map[string]any{"measure": action})
}

func savedResourceInvalid(err error, resource Resource) ToolResult {
	details := resourceValidationDetails(err)
	details["measure"] = "Repair the reported defects in the saved " + resource.Key() + " content only with available tools and authorization. If no safe repair is available, report the blocker and request help. Do not treat it as missing, reinitialize it, or repeat an unchanged read."
	return detailedToolFailure(model.CodeContentInvalid, "Saved "+resource.Key()+" content is invalid: "+err.Error(), details)
}

func candidateContentInvalid(err error, resource Resource) ToolResult {
	details := resourceValidationDetails(err)
	measure := "Correct the reported syntax, structure or business constraints in the proposed " + resource.Key() + " content, then resubmit the corrected edit. No part of this edit was saved."
	if resource.Part == "outline" {
		measure += " " + outlineSourceContract
	}
	details["measure"] = measure
	return detailedToolFailure(model.CodeContentInvalid, "Proposed "+resource.Key()+" content is invalid: "+err.Error(), details)
}

func resourceValidationDetails(err error) map[string]any {
	details := map[string]any{}
	var validation *jsonschema.ValidationError
	if errors.As(err, &validation) {
		issues := []map[string]any{}
		var visit func(*jsonschema.ValidationError)
		visit = func(item *jsonschema.ValidationError) {
			if len(issues) >= 32 {
				details["validation_errors_truncated"] = true
				return
			}
			if len(item.Causes) == 0 {
				issue := map[string]any{"reason": fmt.Sprintf("At %s: %s", argumentPath(item.InstanceLocation), item.Message)}
				issues = append(issues, issue)
				return
			}
			for _, child := range item.Causes {
				visit(child)
			}
		}
		visit(validation)
		details["validation_errors"] = issues
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		details["byte_offset"] = syntax.Offset
	}
	return details
}

func resourceMutationFailure(err error, resource Resource) ToolResult {
	var agentErr *model.AgentError
	if errors.As(err, &agentErr) || errors.Is(err, context.Canceled) {
		return internalToolFailure(err)
	}
	if isFieldEditTool("edit_" + resource.Part) {
		return fieldEditFailure(err, resource)
	}
	var source *pptmutation.ResourceSourceError
	if errors.As(err, &source) {
		if source.Resource == "outline" && errors.Is(source.Err, fs.ErrNotExist) {
			return outlineMissingFailure(resource)
		}
		return resourceReadFailure(source.Err, Resource{Type: "deck", Part: source.Resource})
	}
	switch {
	case errors.Is(err, errResourceUnseen):
		return resourceConflictFailure(resource, true)
	case errors.Is(err, pptmutation.ErrContentConflict):
		return resourceConflictFailure(resource, false)
	case errors.Is(err, pptmutation.ErrOutlineNotExist):
		return outlineMissingFailure(resource)
	case errors.Is(err, pptmutation.ErrSlideNotFound):
		return detailedToolFailure(model.CodeNotFound, "Page "+resource.SlideID+" has been deleted or has not been created in the current outline.", map[string]any{"measure": `Call read_resource with {"type":"outline"} to confirm the page's current id and pass it as slide_id to page tools. Create missing pages with edit_outline when disclosed, and use the returned IDs in a later model turn.`})
	case errors.Is(err, fs.ErrNotExist):
		return resourceReadFailure(err, resource)
	}
	var match *pptmutation.TextEditMatchError
	if errors.As(err, &match) {
		code := model.CodeEditNotFound
		if match.Matches > 1 {
			code = model.CodeEditAmbiguous
		}
		return detailedToolFailure(code, fmt.Sprintf("At /edits/%d/old_text: %s", match.Index, err.Error()), map[string]any{
			"edit_index": match.Index, "match_count": match.Matches,
			"measure": resourceReadAction(resource) + " Choose an anchor that occurs exactly once, including its actual whitespace; a match failure alone does not establish a concurrent update.",
		})
	}
	if !errors.Is(err, pptmutation.ErrInvalid) {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			if pathErr.Op == "read" || pathErr.Op == "open" || pathErr.Op == "stat" {
				return readFailure(err)
			}
			return persistenceToolFailure(err)
		}
		// Validation of pre-existing source is distinct from a rejected edit.
		var syntax *json.SyntaxError
		var validation *jsonschema.ValidationError
		if errors.As(err, &syntax) || errors.As(err, &validation) || errors.Is(err, spec.ErrInvalid) {
			return savedResourceInvalid(err, resource)
		}
		return internalToolFailure(err)
	}
	return candidateContentInvalid(err, resource)
}

func fieldEditFailure(err error, resource Resource) ToolResult {
	if errors.Is(err, pptmutation.ErrOutlineNotExist) {
		return outlineMissingFailure(resource)
	}
	var source *pptmutation.ResourceSourceError
	if errors.As(err, &source) {
		target := resource
		if source.Resource == "outline" {
			target = Resource{Type: "deck", Part: "outline"}
		}
		if errors.Is(source.Err, spec.ErrInvalid) {
			return savedResourceInvalid(source.Err, target)
		}
		if resource.Part == "spec" && source.Resource == "outline" && errors.Is(source.Err, fs.ErrNotExist) {
			return outlineMissingFailure(resource)
		}
		return internalToolFailure(err)
	}
	switch {
	case errors.Is(err, errResourceUnseen):
		return resourceConflictFailure(resource, true)
	case errors.Is(err, pptmutation.ErrContentConflict):
		return resourceConflictFailure(resource, false)
	case resource.Part == "spec" && errors.Is(err, pptmutation.ErrSlideNotFound):
		return resourceReadToolFailure(err, resource)
	case resource.Part == "spec" && errors.Is(err, pptmutation.ErrInvalid):
		return candidateContentInvalid(err, resource)
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) && pathErr.Op != "read" && pathErr.Op != "open" && pathErr.Op != "stat" {
		return persistenceToolFailure(err)
	}
	// Valid field arguments merged with valid saved/default content cannot
	// produce invalid manifest/design content. Such failures are invariants.
	return internalToolFailure(err)
}
