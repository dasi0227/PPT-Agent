package prompt

// Version identifies this coordinated prompt release; Hash identifies exact content.
const Version = "2026-09-23.v2"

const singleCallVersion = "2026-09-29.v1"
const conversationVersion = "2026-09-29.v2"
const reviewVersion = "2026-09-29.v3"

type entry struct{ Path, Version string }

const authoringVersion = "2026-10-06.v1"

var catalog = map[string]entry{
	"command.commit":          {"prompts/command/commit.md", singleCallVersion},
	"command.compact":         {"prompts/command/compact.md", Version},
	"command.handoff":         {"prompts/command/handoff.md", Version},
	"command.polish":          {"prompts/command/polish.md", singleCallVersion},
	"command.rename":          {"prompts/command/rename.md", singleCallVersion},
	"core.agent":              {"prompts/core/agent.md", conversationVersion},
	"core.output":             {"prompts/core/output.md", Version},
	"core.quality":            {"prompts/core/quality.md", conversationVersion},
	"core.reference":          {"prompts/core/reference.md", authoringVersion},
	"mode.chat":               {"prompts/mode/chat.md", conversationVersion},
	"mode.execute":            {"prompts/mode/execute.md", authoringVersion},
	"mode.grill":              {"prompts/mode/grill.md", conversationVersion},
	"mode.plan":               {"prompts/mode/plan.md", Version},
	"workflow.execution":      {"prompts/workflow/execution.md", authoringVersion},
	"workflow.completion":     {"prompts/workflow/completion.md", authoringVersion},
	"workflow.recovery":       {"prompts/workflow/recovery.md", authoringVersion},
	"subagent.reviewer.agent": {"prompts/subagent/reviewer/agent.md", reviewVersion},
	"playbook.init":           {"prompts/playbook/init.md", authoringVersion},
	"playbook.improve":        {"prompts/playbook/improve.md", authoringVersion},
	"playbook.html":           {"prompts/playbook/html.md", authoringVersion},
	"playbook.runtime":        {"prompts/playbook/runtime.md", authoringVersion},
}
