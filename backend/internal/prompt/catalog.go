package prompt

// Version identifies this coordinated prompt release; Hash identifies exact content.
const Version = "2026-09-23.v2"

const singleCallVersion = "2026-09-29.v1"
const conversationVersion = "2026-10-16.v1"
const reviewVersion = "2026-10-15.v1"

type entry struct{ Path, Version string }

const authoringVersion = "2026-10-16.v1"
const toolVersion = "2026-10-16.v1"
const referenceVersion = "2026-10-15.v1"
const protocolVersion = "2026-10-18.v1"
const toolDescriptionVersion = "2026-10-31.v1"
const intentVersion = "2026-10-31.v1"
const pruneVersion = "2026-10-31.v1"
const renameDecisionVersion = "2026-10-31.v1"

var catalog = map[string]entry{
	"command.commit":           {"prompts/command/commit.md", singleCallVersion},
	"command.compact":          {"prompts/command/compact.md", Version},
	"command.handoff":          {"prompts/command/handoff.md", Version},
	"command.polish":           {"prompts/command/polish.md", singleCallVersion},
	"command.suggest":          {"prompts/command/suggest.md", "2026-10-16.v1"},
	"command.rename":           {"prompts/command/rename.md", singleCallVersion},
	"core.agent":               {"prompts/core/agent.md", conversationVersion},
	"core.references":          {"prompts/core/references.md", referenceVersion},
	"core.protocols":           {"prompts/core/protocols.md", protocolVersion},
	"core.contracts":           {"prompts/core/contracts.md", authoringVersion},
	"jev.intent":               {"prompts/jev/intent.md", intentVersion},
	"jev.prune":                {"prompts/jev/prune.md", pruneVersion},
	"jev.rename":               {"prompts/jev/rename.md", renameDecisionVersion},
	"mode.chat":                {"prompts/mode/chat.md", conversationVersion},
	"mode.create":              {"prompts/mode/create.md", authoringVersion},
	"mode.ask":                 {"prompts/mode/ask.md", conversationVersion},
	"mode.plan":                {"prompts/mode/plan.md", Version},
	"subagent.reviewer.agent":  {"prompts/subagent/reviewer/agent.md", reviewVersion},
	"subagent.reviewer.output": {"prompts/subagent/reviewer/output.md", reviewVersion},
	"playbook.tool":            {"prompts/playbook/tool.md", toolVersion},
	"playbook.spec":            {"prompts/playbook/spec.md", authoringVersion},
	"playbook.html":            {"prompts/playbook/html.md", authoringVersion},
	"rule.authoring":           {"prompts/rule/authoring.md", authoringVersion},
	"rule.styling":             {"prompts/rule/styling.md", authoringVersion},
	"tool.load_component":      {"prompts/tool/load_component.md", toolDescriptionVersion},
	"tool.load_skill":          {"prompts/tool/load_skill.md", toolDescriptionVersion},
	"tool.read_resource":       {"prompts/tool/read_resource.md", toolDescriptionVersion},
	"tool.read_image":          {"prompts/tool/read_image.md", toolDescriptionVersion},
	"tool.render_slide":        {"prompts/tool/render_slide.md", toolDescriptionVersion},
	"tool.edit_manifest":       {"prompts/tool/edit_manifest.md", toolDescriptionVersion},
	"tool.edit_design":         {"prompts/tool/edit_design.md", toolDescriptionVersion},
	"tool.edit_spec":           {"prompts/tool/edit_spec.md", toolDescriptionVersion},
	"tool.edit_outline":        {"prompts/tool/edit_outline.md", toolDescriptionVersion},
	"tool.edit_html":           {"prompts/tool/edit_html.md", toolDescriptionVersion},
	"tool.create_plan":         {"prompts/tool/create_plan.md", toolDescriptionVersion},
	"tool.update_plan":         {"prompts/tool/update_plan.md", toolDescriptionVersion},
	"tool.request_privilege":   {"prompts/tool/request_privilege.md", toolDescriptionVersion},
	"tool.ask_user":            {"prompts/tool/ask_user.md", toolDescriptionVersion},
	"tool.review_ppt":          {"prompts/tool/review_ppt.md", toolDescriptionVersion},
	"tool.suggest_request":     {"prompts/tool/suggest_request.md", toolDescriptionVersion},
	"tool.git_commit":          {"prompts/tool/git_commit.md", toolDescriptionVersion},
	"tool.run_command":         {"prompts/tool/run_command.md", toolDescriptionVersion},
	"tool.rename_thread":       {"prompts/tool/rename_thread.md", toolDescriptionVersion},
	"tool.compact_context":     {"prompts/tool/compact_context.md", toolDescriptionVersion},
	"tool.polish_instruction":  {"prompts/tool/polish_instruction.md", toolDescriptionVersion},
	"tool.handoff_thread":      {"prompts/tool/handoff_thread.md", toolDescriptionVersion},
}
