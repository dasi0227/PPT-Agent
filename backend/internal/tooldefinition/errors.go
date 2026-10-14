package tooldefinition

import (
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// These bind business error descriptions only. Runtime and submission-envelope
// failures still return their registered codes without being tool documentation.
var businessErrorContracts = map[string][]string{
	"read_resource":      {"RESOURCE_INVALID", "RESOURCE_NOT_FOUND"},
	"read_image":         {"IMAGE_INVALID", "IMAGE_NOT_FOUND"},
	"render_slide":       {"CONTENT_CONFLICT", "RENDER_FAILED", "RESOURCE_INVALID", "RESOURCE_NOT_FOUND"},
	"load_component":     {"COMPONENT_NOT_FOUND"},
	"load_skill":         {"SKILL_NOT_FOUND"},
	"edit_manifest":      {"CONTENT_CONFLICT", "RESOURCE_INVALID"},
	"edit_design":        {"CONTENT_CONFLICT", "RESOURCE_INVALID"},
	"edit_spec":          {"CONTENT_CONFLICT", "CONTENT_INVALID", "RESOURCE_INVALID", "RESOURCE_NOT_FOUND"},
	"edit_html":          {"CONTENT_CONFLICT", "CONTENT_INVALID", "CONTENT_TOO_LARGE", "EDIT_ANCHOR_AMBIGUOUS", "EDIT_ANCHOR_NOT_FOUND", "RESOURCE_CONTENT_INVALID", "RESOURCE_NOT_FOUND"},
	"edit_outline":       {"CONTENT_CONFLICT", "CONTENT_INVALID", "CONTENT_TOO_LARGE", "EDIT_ANCHOR_AMBIGUOUS", "EDIT_ANCHOR_NOT_FOUND", "OUTLINE_NOT_INITIALIZED", "RESOURCE_CONTENT_INVALID", "TARGET_ALREADY_EXISTS"},
	"git_commit":         {"GIT_COMMIT_FAILED"},
	"run_command":        {"COMMAND_EXIT_NONZERO", "COMMAND_FLAG_DENIED", "COMMAND_NOT_ALLOWED", "COMMAND_NOT_AVAILABLE", "COMMAND_OUTPUT_LIMIT", "COMMAND_PARSE_INVALID", "COMMAND_PATH_INVALID", "COMMAND_SYNTAX_DENIED", "COMMAND_TIMEOUT", "CONTENT_CONFLICT", "CONTENT_INVALID", "OUTLINE_NOT_INITIALIZED", "RESOURCE_CONTENT_INVALID", "RESOURCE_NOT_FOUND"},
	"create_plan":        {"PLAN_INVALID"},
	"update_plan":        {"PLAN_INVALID"},
	"request_privilege":  nil,
	"ask_user":           nil,
	"review_task":        {"REVIEW_FAILED"},
	"finish_task":        {"COMPLETION_GATE_BLOCK"},
	"submit_review":      nil,
	"rename_thread":      nil,
	"compact_context":    nil,
	"polish_instruction": nil,
	"handoff_thread":     nil,
	"git_commit.command": nil,
}

func validateBusinessErrorCodes(id string, codes []string) error {
	known := map[string]bool{}
	for _, code := range model.RegisteredErrorCodes() {
		known[code] = true
	}
	for _, code := range codes {
		if model.IsCommonErrorCode(code) {
			return fmt.Errorf("tool %q declares common error code %q as a business error", id, code)
		}
		if !known[code] {
			return fmt.Errorf("tool %q declares unknown public error code %q", id, code)
		}
	}
	return nil
}
