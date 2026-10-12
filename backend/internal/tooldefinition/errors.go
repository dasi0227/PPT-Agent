package tooldefinition

import (
	"fmt"
	"sort"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

const CodeResourceEditApprovalRequired = "RESOURCE_EDIT_APPROVAL_REQUIRED"

// These are the existing submission-validation codes, emitted in tool replies.
var submissionCodes = []string{"TOOL_COUNT", "TOOL_NAME", "HAS_TEXT", "INVALID_SUBMISSION", "INVALID_FIELDS", "INVALID_TITLE", "INVALID_CONTENT", "INVALID_ARGUMENTS", "INVALID_ACTION", "KEEP_FIELDS", "RENAME_FIELDS"}

var runtimeCodes = []string{"RUN_CANCELED", "DEPENDENCY_FAILED", "RESOURCE_NOT_DISCLOSED", "TOOL_ARGUMENT_INVALID", "INVALID_CONTROL_CALL", "INTERNAL", "IDEMPOTENCY_KEY_REUSED"}
var domainCodes = union(runtimeCodes, []string{"CAPABILITY_DENIED", "TARGET_OUT_OF_SCOPE"})
var writeCodes = union(domainCodes, []string{"RUN_SESSION_MISSING", "COMMIT_FAILED"})
var resourceMutationCodes = []string{"RESOURCE_INVALID", "RESOURCE_NOT_FOUND", "RESOURCE_CONTENT_INVALID", "CONTENT_TOO_LARGE", "CONTENT_INVALID", "CONTENT_CONFLICT", "OUTLINE_NOT_INITIALIZED", "READ_FAILED", "WRITE_FAILED"}
var submissionEnvelopeCodes = []string{"TOOL_COUNT", "TOOL_NAME", "HAS_TEXT", "INVALID_SUBMISSION"}

// Scope is model-visible failed tool replies, not HTTP or whole-run failures.
var errorContracts = map[string][]string{
	"read_resource":      union(domainCodes, []string{"RESOURCE_INVALID", "RESOURCE_NOT_FOUND", "RESOURCE_CONTENT_INVALID", "CONTENT_TOO_LARGE", "OUTLINE_NOT_INITIALIZED", "READ_FAILED"}),
	"render_slide":       union(domainCodes, []string{"CONTENT_INVALID", "RENDER_FAILED", "RENDER_WORKER_UNAVAILABLE"}),
	"read_image":         union(domainCodes, []string{"RESOURCE_INVALID", "RESOURCE_NOT_FOUND", "ATTACHMENT_INVALID", "ATTACHMENT_NOT_FOUND"}),
	"load_component":     union(domainCodes, []string{"RESOURCE_NOT_FOUND", "CONTENT_TOO_LARGE"}),
	"load_skill":         union(domainCodes, []string{"RESOURCE_NOT_FOUND", "CONTENT_TOO_LARGE", "CONTENT_INVALID", "RUN_SESSION_MISSING", "SKILL_NOT_FOUND", "SKILL_DISABLED", "SKILL_SELECTION_INVALID"}),
	"edit_manifest":      union(writeCodes, resourceMutationCodes, []string{"AGENT_FAILED"}),
	"edit_design":        union(writeCodes, resourceMutationCodes, []string{"AGENT_FAILED"}),
	"edit_spec":          union(writeCodes, resourceMutationCodes),
	"edit_html":          union(writeCodes, resourceMutationCodes, []string{"EDIT_ANCHOR_NOT_FOUND", "EDIT_ANCHOR_AMBIGUOUS"}),
	"edit_outline":       union(writeCodes, resourceMutationCodes, []string{"EDIT_ANCHOR_NOT_FOUND", "EDIT_ANCHOR_AMBIGUOUS", "TARGET_ALREADY_EXISTS", "AGENT_FAILED"}),
	"git_commit":         union(writeCodes, []string{"GIT_COMMIT_FAILED"}),
	"run_command":        union(writeCodes, resourceMutationCodes, []string{CodeResourceEditApprovalRequired, "COMMAND_PARSE_INVALID", "COMMAND_SYNTAX_DENIED", "COMMAND_NOT_ALLOWED", "COMMAND_NOT_AVAILABLE", "COMMAND_FLAG_DENIED", "COMMAND_PATH_OUTSIDE_PROJECT", "COMMAND_PATH_INVALID", "COMMAND_SENSITIVE_READ_DENIED", "COMMAND_EXIT_NONZERO", "COMMAND_TIMEOUT", "COMMAND_OUTPUT_LIMIT", "COMMAND_EXEC_FAILED", "COMMAND_INVARIANT_VIOLATION", "EDIT_ANCHOR_NOT_FOUND", "EDIT_ANCHOR_AMBIGUOUS"}),
	"create_plan":        union(runtimeCodes, []string{"PLAN_INVALID", "AGENT_FAILED"}),
	"update_plan":        union(runtimeCodes, []string{"PLAN_INVALID", "AGENT_FAILED"}),
	"request_privilege":  runtimeCodes,
	"ask_user":           runtimeCodes,
	"review_task":        union(runtimeCodes, []string{"REVIEW_FAILED"}),
	"finish_task":        union(runtimeCodes, []string{"FINISH_MESSAGE_EMPTY", "COMPLETION_GATE_BLOCK"}),
	"submit_review":      {"TOOL_COUNT", "TOOL_NAME", "INVALID_SUBMISSION", "INVALID_ARGUMENTS"},
	"rename_thread":      union(submissionEnvelopeCodes, []string{"INVALID_ACTION", "KEEP_FIELDS", "RENAME_FIELDS", "INVALID_TITLE"}),
	"compact_context":    union(submissionEnvelopeCodes, []string{"INVALID_FIELDS", "INVALID_TITLE", "INVALID_CONTENT"}),
	"polish_instruction": union(submissionEnvelopeCodes, []string{"INVALID_FIELDS", "INVALID_TITLE", "INVALID_CONTENT"}),
	"handoff_thread":     union(submissionEnvelopeCodes, []string{"INVALID_FIELDS", "INVALID_TITLE", "INVALID_CONTENT"}),
	"git_commit.command": union(submissionEnvelopeCodes, []string{"INVALID_ARGUMENTS"}),
}

func union(groups ...[]string) []string {
	set := map[string]bool{}
	for _, group := range groups {
		for _, code := range group {
			set[code] = true
		}
	}
	result := make([]string, 0, len(set))
	for code := range set {
		result = append(result, code)
	}
	sort.Strings(result)
	return result
}

func validateErrorCodes(id string, codes []string) error {
	known := map[string]bool{}
	for _, code := range model.RegisteredErrorCodes() {
		known[code] = true
	}
	for _, code := range submissionCodes {
		known[code] = true
	}
	// This existing preflight code intentionally retains its current Runtime category.
	known[CodeResourceEditApprovalRequired] = true
	for _, code := range codes {
		if !known[code] {
			return fmt.Errorf("tool %q declares unknown public error code %q", id, code)
		}
	}
	return nil
}
