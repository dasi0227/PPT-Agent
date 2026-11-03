package prompt

// Version identifies this coordinated prompt release; Hash identifies exact content.
const Version = "2026-09-23.v2"

const singleCallVersion = "2026-09-29.v1"
const conversationVersion = "2026-10-16.v1"

type entry struct{ Path, Version string }

const authoringVersion = "2026-10-16.v1"
const toolVersion = "2026-10-16.v1"
const protocolVersion = "2026-10-18.v1"
const intentVersion = "2026-11-02.v2"
const pruneVersion = "2026-10-31.v1"
const renameDecisionVersion = "2026-10-31.v1"
const previewDecisionVersion = "2026-11-01.v1"
const resourceVersion = "2026-11-01.v1"

var catalog = map[string]entry{
	"reminder_constraint_boundaries": {"prompts/reminder/boundaries.md", resourceVersion},
	"command_commit":                 {"prompts/command/commit.md", singleCallVersion},
	"command_compact":                {"prompts/command/compact.md", Version},
	"command_handoff":                {"prompts/command/handoff.md", resourceVersion},
	"command_polish":                 {"prompts/command/polish.md", singleCallVersion},
	"command_suggest":                {"prompts/command/suggest.md", resourceVersion},
	"command_rename":                 {"prompts/command/rename.md", singleCallVersion},
	"decision_preview":               {"prompts/jev/preview.md", previewDecisionVersion},
	"core_agent":                     {"prompts/core/agent.md", resourceVersion},
	"core_references":                {"prompts/core/references.md", resourceVersion},
	"core_protocols":                 {"prompts/core/protocols.md", protocolVersion},
	"core_contracts":                 {"prompts/core/contracts.md", authoringVersion},
	"jev_intent":                     {"prompts/jev/intent.md", intentVersion},
	"jev_prune":                      {"prompts/jev/prune.md", pruneVersion},
	"jev_rename":                     {"prompts/jev/rename.md", renameDecisionVersion},
	"mode_chat":                      {"prompts/mode/chat.md", conversationVersion},
	"mode_create":                    {"prompts/mode/create.md", authoringVersion},
	"mode_ask":                       {"prompts/mode/ask.md", conversationVersion},
	"mode_plan":                      {"prompts/mode/plan.md", Version},
	"reviewer_agent":                 {"prompts/subagent/reviewer/agent.md", resourceVersion},
	"playbook_tool":                  {"prompts/playbook/tool.md", toolVersion},
	"playbook_spec":                  {"prompts/playbook/spec.md", authoringVersion},
	"playbook_html":                  {"prompts/playbook/html.md", resourceVersion},
	"rule_authoring":                 {"prompts/rule/authoring.md", authoringVersion},
	"rule_styling":                   {"prompts/rule/styling.md", authoringVersion},
	"rule_commentating":              {"prompts/rule/commentating.md", "2026-11-03.v1"},
}
