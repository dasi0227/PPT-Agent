package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

const outlineSourceContract = "Sections require title, purpose, slides and subsections; subsections require title, purpose and slides. A new slide contains only title (no id, slide_id or purpose). Existing slides contain only slide_id and title. Omit IDs for all new nodes; preserve existing identities. Use either direct slides or subsections per section, never both."

func resourceReadFailure(err error, resource Resource) ToolResult {
	if errors.Is(err, spec.ErrInvalid) {
		return savedResourceInvalid(err)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return readFailure(err)
	}
	if resource.Part == "outline" {
		return failedToolResult("OUTLINE_NOT_INITIALIZED", "outline is not initialized", false)
	}
	action := "The project resource is missing. Report the missing resource; do not invent a replacement or repeat an unchanged read."
	switch resource.Part {
	case "html":
		action = "If this slide_id exists in the current outline, create its HTML with write_html when disclosed, then render_slide. Otherwise read the outline and select an existing slide_id. Do not patch, render or read missing HTML repeatedly."
	case "spec":
		action = "If this slide_id exists in the current outline, create its spec with edit_spec, supplying key_message and elements, when disclosed. Otherwise read the outline and select an existing slide_id. Do not repeat an unchanged read."
	}
	return detailedToolFailure(CodeResourceNotFound, resource.Part+" content was not found", map[string]any{"next_action": action})
}

func resourceValidationDetails(err error) map[string]any {
	details := map[string]any{}
	var validation *jsonschema.ValidationError
	if errors.As(err, &validation) {
		leaf := mostSpecificValidationError(validation)
		details["field"] = argumentPath(leaf.InstanceLocation)
		issues := []map[string]any{}
		var visit func(*jsonschema.ValidationError)
		visit = func(item *jsonschema.ValidationError) {
			if len(issues) >= 32 {
				details["validation_errors_truncated"] = true
				return
			}
			if len(item.Causes) == 0 {
				issue := map[string]any{"field": argumentPath(item.InstanceLocation), "reason": item.Message}
				if strings.HasSuffix(item.KeywordLocation, "/type") {
					expected, actual, ok := strings.Cut(strings.TrimPrefix(item.Message, "expected "), ", but got ")
					if ok {
						issue["expected"], issue["actual"] = expected, actual
					}
				}
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
	var field *pptmutation.SourceFieldError
	if errors.As(err, &field) {
		details["field"] = field.Field
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		details["byte_offset"] = syntax.Offset
	}
	return details
}

func savedResourceInvalid(err error) ToolResult {
	return detailedToolFailure("RESOURCE_CONTENT_INVALID", err.Error(), resourceValidationDetails(err))
}

func resourceMutationFailure(err error, resource Resource) ToolResult {
	switch {
	case errors.Is(err, pptmutation.ErrContentConflict):
		return failedToolResult(CodeContentConflict, err.Error(), false)
	case errors.Is(err, pptmutation.ErrOutlineExists):
		return detailedToolFailure("TARGET_ALREADY_EXISTS", err.Error(), map[string]any{"next_action": "Read the existing outline with read_resource(resource: outline), then use arrange_outline with exact text edits. Never reinitialize it."})
	case errors.Is(err, pptmutation.ErrOutlineNotInitialized):
		return failedToolResult("OUTLINE_NOT_INITIALIZED", err.Error(), false)
	case errors.Is(err, pptmutation.ErrSlideNotFound):
		return detailedToolFailure(CodeResourceNotFound, err.Error(), map[string]any{"field": "/slide_id", "next_action": "Read the current outline and use an existing Runtime-issued slide_id. If no outline exists, use init_outline when disclosed before authoring pages."})
	case errors.Is(err, fs.ErrNotExist):
		return resourceReadFailure(err, resource)
	}
	var match *pptmutation.TextEditMatchError
	if errors.As(err, &match) {
		code := "EDIT_ANCHOR_NOT_FOUND"
		if match.Matches > 1 {
			code = "EDIT_ANCHOR_AMBIGUOUS"
		}
		return detailedToolFailure(code, err.Error(), map[string]any{
			"field": fmt.Sprintf("/edits/%d/old_text", match.Index), "edit_index": match.Index, "match_count": match.Matches,
		})
	}
	if !errors.Is(err, pptmutation.ErrInvalid) {
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			if pathErr.Op == "read" || pathErr.Op == "open" || pathErr.Op == "stat" {
				return readFailure(err)
			}
			return writeFailure(err)
		}
		// Validation of pre-existing source is distinct from a rejected edit.
		var syntax *json.SyntaxError
		var validation *jsonschema.ValidationError
		if errors.As(err, &syntax) || errors.As(err, &validation) || errors.Is(err, spec.ErrInvalid) {
			return savedResourceInvalid(err)
		}
		return writeFailure(err)
	}
	details := resourceValidationDetails(err)
	if resource.Part == "outline" {
		details["next_action"] = "Correct the reported fields in the outline JSON source. " + outlineSourceContract + " No part of this edit was saved."
	}
	return detailedToolFailure(CodeContentInvalid, err.Error(), details)
}
