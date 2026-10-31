package prompt

// Version identifies this coordinated prompt release; Hash identifies exact content.
const Version = "2026-09-23.v2"

const singleCallVersion = "2026-09-29.v1"
const conversationVersion = "2026-10-16.v1"
const reviewVersion = "2026-10-31.v1"

type entry struct{ Path, Version string }

const authoringVersion = "2026-10-16.v1"
const toolVersion = "2026-10-16.v1"
const referenceVersion = "2026-10-15.v1"
const protocolVersion = "2026-10-18.v1"
const toolDescriptionVersion = "2026-10-31.v1"
const intentVersion = "2026-10-31.v1"
const pruneVersion = "2026-10-31.v1"
const renameDecisionVersion = "2026-10-31.v1"
const previewDecisionVersion = "2026-10-31.v1"

var catalog = map[string]entry{
	"requirements_prohibitions_boundaries": {"prompts/addition/boundaries.md", "2026-10-31.v1"},
	"command_commit":                       {"prompts/command/commit.md", singleCallVersion},
	"command_compact":                      {"prompts/command/compact.md", Version},
	"command_handoff":                      {"prompts/command/handoff.md", Version},
	"command_polish":                       {"prompts/command/polish.md", singleCallVersion},
	"command_suggest":                      {"prompts/command/suggest.md", "2026-10-16.v1"},
	"command_rename":                       {"prompts/command/rename.md", singleCallVersion},
	"decision_preview":                     {"prompts/decision/preview.md", previewDecisionVersion},
	"core_agent":                           {"prompts/core/agent.md", conversationVersion},
	"core_references":                      {"prompts/core/references.md", referenceVersion},
	"core_protocols":                       {"prompts/core/protocols.md", protocolVersion},
	"core_contracts":                       {"prompts/core/contracts.md", authoringVersion},
	"jev_intent":                           {"prompts/jev/intent.md", intentVersion},
	"jev_prune":                            {"prompts/jev/prune.md", pruneVersion},
	"jev_rename":                           {"prompts/jev/rename.md", renameDecisionVersion},
	"mode_chat":                            {"prompts/mode/chat.md", conversationVersion},
	"mode_create":                          {"prompts/mode/create.md", authoringVersion},
	"mode_ask":                             {"prompts/mode/ask.md", conversationVersion},
	"mode_plan":                            {"prompts/mode/plan.md", Version},
	"subagent_reviewer_agent":              {"prompts/subagent/reviewer/agent.md", reviewVersion},
	"playbook_tool":                        {"prompts/playbook/tool.md", toolVersion},
	"playbook_spec":                        {"prompts/playbook/spec.md", authoringVersion},
	"playbook_html":                        {"prompts/playbook/html.md", authoringVersion},
	"rule_authoring":                       {"prompts/rule/authoring.md", authoringVersion},
	"rule_styling":                         {"prompts/rule/styling.md", authoringVersion},
	"tool_load_component":                  {"prompts/tool/load_component.md", toolDescriptionVersion},
	"tool_load_skill":                      {"prompts/tool/load_skill.md", toolDescriptionVersion},
	"tool_read_resource":                   {"prompts/tool/read_resource.md", toolDescriptionVersion},
	"tool_read_image":                      {"prompts/tool/read_image.md", toolDescriptionVersion},
	"tool_render_slide":                    {"prompts/tool/render_slide.md", toolDescriptionVersion},
	"tool_edit_manifest":                   {"prompts/tool/edit_manifest.md", toolDescriptionVersion},
	"tool_edit_design":                     {"prompts/tool/edit_design.md", toolDescriptionVersion},
	"tool_edit_spec":                       {"prompts/tool/edit_spec.md", toolDescriptionVersion},
	"tool_edit_outline":                    {"prompts/tool/edit_outline.md", toolDescriptionVersion},
	"tool_edit_html":                       {"prompts/tool/edit_html.md", toolDescriptionVersion},
	"tool_create_plan":                     {"prompts/tool/create_plan.md", toolDescriptionVersion},
	"tool_update_plan":                     {"prompts/tool/update_plan.md", toolDescriptionVersion},
	"tool_request_privilege":               {"prompts/tool/request_privilege.md", toolDescriptionVersion},
	"tool_ask_user":                        {"prompts/tool/ask_user.md", toolDescriptionVersion},
	"tool_review_ppt":                      {"prompts/tool/review_ppt.md", toolDescriptionVersion},
	"tool_suggest_request":                 {"prompts/tool/suggest_request.md", toolDescriptionVersion},
	"tool_git_commit":                      {"prompts/tool/git_commit.md", toolDescriptionVersion},
	"tool_run_command":                     {"prompts/tool/run_command.md", toolDescriptionVersion},
	"tool_rename_thread":                   {"prompts/tool/rename_thread.md", toolDescriptionVersion},
	"tool_compact_context":                 {"prompts/tool/compact_context.md", toolDescriptionVersion},
	"tool_polish_instruction":              {"prompts/tool/polish_instruction.md", toolDescriptionVersion},
	"tool_handoff_thread":                  {"prompts/tool/handoff_thread.md", toolDescriptionVersion},
}
