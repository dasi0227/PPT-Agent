package tooldefinition

import (
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

const CodeResourceEditApprovalRequired = "RESOURCE_EDIT_APPROVAL_REQUIRED"

// These bind business error descriptions only. Runtime and submission-envelope
// failures still return their registered codes without being tool documentation.
var businessErrorContracts = map[string][]string{
	"read_resource":  {"CONTENT_TOO_LARGE", "OUTLINE_NOT_INITIALIZED", "READ_FAILED", "RESOURCE_CONTENT_INVALID", "RESOURCE_INVALID", "RESOURCE_NOT_FOUND"},
	"render_slide":   {"CONTENT_INVALID", "READ_FAILED", "RENDER_FAILED", "RENDER_WORKER_UNAVAILABLE", "RESOURCE_NOT_FOUND"},
	"read_image":     {"ATTACHMENT_INVALID", "ATTACHMENT_NOT_FOUND", "RESOURCE_INVALID", "RESOURCE_NOT_FOUND"},
	"load_component": {"CONTENT_TOO_LARGE", "RESOURCE_NOT_FOUND"},
	"load_skill":     {"CONTENT_INVALID", "CONTENT_TOO_LARGE", "RESOURCE_NOT_FOUND"},
	"edit_manifest":  {"CONTENT_CONFLICT", "CONTENT_INVALID", "CONTENT_TOO_LARGE", "READ_FAILED", "RESOURCE_CONTENT_INVALID", "RESOURCE_NOT_FOUND", "WRITE_FAILED"},
	"edit_design":    {"CONTENT_CONFLICT", "CONTENT_INVALID", "CONTENT_TOO_LARGE", "READ_FAILED", "RESOURCE_CONTENT_INVALID", "RESOURCE_NOT_FOUND", "WRITE_FAILED"},
	"edit_spec":      {"CONTENT_CONFLICT", "CONTENT_INVALID", "CONTENT_TOO_LARGE", "READ_FAILED", "RESOURCE_CONTENT_INVALID", "RESOURCE_NOT_FOUND", "WRITE_FAILED"},
	"edit_html":      {"CONTENT_CONFLICT", "CONTENT_INVALID", "CONTENT_TOO_LARGE", "EDIT_ANCHOR_AMBIGUOUS", "EDIT_ANCHOR_NOT_FOUND", "READ_FAILED", "RESOURCE_CONTENT_INVALID", "RESOURCE_NOT_FOUND", "WRITE_FAILED"},
	"edit_outline":   {"CONTENT_CONFLICT", "CONTENT_INVALID", "CONTENT_TOO_LARGE", "EDIT_ANCHOR_AMBIGUOUS", "EDIT_ANCHOR_NOT_FOUND", "OUTLINE_NOT_INITIALIZED", "READ_FAILED", "RESOURCE_CONTENT_INVALID", "RESOURCE_NOT_FOUND", "TARGET_ALREADY_EXISTS", "WRITE_FAILED"},
	"git_commit":     {"GIT_COMMIT_FAILED"},
	"run_command": {
		"COMMAND_EXEC_FAILED", "COMMAND_EXIT_NONZERO", "COMMAND_FLAG_DENIED", "COMMAND_NOT_ALLOWED", "COMMAND_NOT_AVAILABLE", "COMMAND_OUTPUT_LIMIT",
		"COMMAND_PARSE_INVALID", "COMMAND_PATH_INVALID", "COMMAND_PATH_OUTSIDE_PROJECT", "COMMAND_SENSITIVE_READ_DENIED", "COMMAND_SYNTAX_DENIED", "COMMAND_TIMEOUT",
		"CONTENT_CONFLICT", CodeResourceEditApprovalRequired, "RESOURCE_NOT_FOUND",
	},
	"create_plan":        {"PLAN_INVALID"},
	"update_plan":        {"PLAN_INVALID"},
	"request_privilege":  nil,
	"ask_user":           nil,
	"review_task":        {"REVIEW_FAILED"},
	"finish_task":        {"COMPLETION_GATE_BLOCK", "FINISH_MESSAGE_EMPTY"},
	"submit_review":      {"INVALID_ARGUMENTS"},
	"rename_thread":      {"INVALID_ACTION", "INVALID_TITLE", "KEEP_FIELDS", "RENAME_FIELDS"},
	"compact_context":    {"INVALID_CONTENT", "INVALID_FIELDS", "INVALID_TITLE"},
	"polish_instruction": {"INVALID_CONTENT", "INVALID_FIELDS", "INVALID_TITLE"},
	"handoff_thread":     {"INVALID_CONTENT", "INVALID_FIELDS", "INVALID_TITLE"},
	"git_commit.command": {"INVALID_ARGUMENTS"},
}

func validateBusinessErrorCodes(id string, codes []string) error {
	known := map[string]bool{}
	for _, code := range model.RegisteredErrorCodes() {
		known[code] = true
	}
	for _, code := range []string{"INVALID_FIELDS", "INVALID_TITLE", "INVALID_CONTENT", "INVALID_ARGUMENTS", "INVALID_ACTION", "KEEP_FIELDS", "RENAME_FIELDS"} {
		known[code] = true
	}
	// This existing preflight code intentionally retains its current Runtime category.
	known[CodeResourceEditApprovalRequired] = true
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
