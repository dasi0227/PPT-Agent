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
	"reminder_constraint_boundaries": {"reminder/boundaries.md", resourceVersion},
	"command_commit":                 {"command/commit.md", singleCallVersion},
	"command_compact":                {"command/compact.md", Version},
	"command_handoff":                {"command/handoff.md", resourceVersion},
	"command_polish":                 {"command/polish.md", singleCallVersion},
	"command_suggest":                {"command/suggest.md", resourceVersion},
	"command_rename":                 {"command/rename.md", singleCallVersion},
	"decision_preview":               {"jev/preview.md", previewDecisionVersion},
	"core_agent":                     {"core/agent.md", resourceVersion},
	"core_references":                {"core/references.md", resourceVersion},
	"core_protocols":                 {"core/protocols.md", protocolVersion},
	"core_contracts":                 {"core/contracts.md", authoringVersion},
	"jev_intent":                     {"jev/intent.md", intentVersion},
	"jev_prune":                      {"jev/prune.md", pruneVersion},
	"jev_rename":                     {"jev/rename.md", renameDecisionVersion},
	"mode_chat":                      {"mode/chat.md", conversationVersion},
	"mode_create":                    {"mode/create.md", authoringVersion},
	"mode_ask":                       {"mode/ask.md", conversationVersion},
	"mode_plan":                      {"mode/plan.md", Version},
	"reviewer_agent":                 {"subagent/reviewer/agent.md", resourceVersion},
	"playbook_tool":                  {"playbook/tool.md", toolVersion},
	"playbook_spec":                  {"playbook/spec.md", authoringVersion},
	"playbook_html":                  {"playbook/html.md", resourceVersion},
	"rule_authoring":                 {"rule/authoring.md", authoringVersion},
	"rule_styling":                   {"rule/styling.md", authoringVersion},
	"rule_commentating":              {"rule/commentating.md", "2026-11-03.v1"},
}
