package tooldefinition

import (
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// These bind business error descriptions only. Runtime and submission-envelope
// failures still return their registered codes without being tool documentation.
var businessErrorContracts = map[string][]string{
	"read_resource":      {},
	"read_image":         {"IMAGE_INVALID"},
	"render_slide":       {"RENDER_FAILED"},
	"load_component":     {},
	"load_skill":         {},
	"edit_manifest":      {},
	"edit_design":        {},
	"edit_spec":          {},
	"edit_html":          {"EDIT_ANCHOR_AMBIGUOUS", "EDIT_ANCHOR_NOT_FOUND"},
	"edit_outline":       {"EDIT_ANCHOR_AMBIGUOUS", "EDIT_ANCHOR_NOT_FOUND", "OUTLINE_ALREADY_EXISTS"},
	"git_commit":         {"GIT_COMMIT_FAILED"},
	"run_command":        {"COMMAND_EXIT_NONZERO", "COMMAND_FLAG_DENIED", "COMMAND_NOT_ALLOWED", "COMMAND_NOT_AVAILABLE", "COMMAND_OUTPUT_LIMIT", "COMMAND_PARSE_INVALID", "COMMAND_PATH_INVALID", "COMMAND_SYNTAX_DENIED", "COMMAND_TIMEOUT"},
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
