package prompt

// Version identifies this coordinated prompt release; Hash identifies exact content.
const Version = "2026-09-23.v2"

const singleCallVersion = "2026-09-29.v1"
const conversationVersion = "2026-09-29.v2"
const reviewVersion = "2026-10-15.v1"

type entry struct{ Path, Version string }

const authoringVersion = "2026-10-06.v1"
const retryVersion = "2026-10-12.v4"

var catalog = map[string]entry{
	"command.commit":           {"prompts/command/commit.md", singleCallVersion},
	"command.compact":          {"prompts/command/compact.md", Version},
	"command.handoff":          {"prompts/command/handoff.md", Version},
	"command.polish":           {"prompts/command/polish.md", singleCallVersion},
	"command.rename":           {"prompts/command/rename.md", singleCallVersion},
	"core.agent":               {"prompts/core/agent.md", conversationVersion},
	"core.output":              {"prompts/core/output.md", Version},
	"core.quality":             {"prompts/core/quality.md", conversationVersion},
	"core.reference":           {"prompts/core/reference.md", authoringVersion},
	"mode.chat":                {"prompts/mode/chat.md", conversationVersion},
	"mode.execute":             {"prompts/mode/execute.md", authoringVersion},
	"mode.ask":                 {"prompts/mode/ask.md", conversationVersion},
	"mode.plan":                {"prompts/mode/plan.md", Version},
	"subagent.reviewer.agent":  {"prompts/subagent/reviewer/agent.md", reviewVersion},
	"subagent.reviewer.output": {"prompts/subagent/reviewer/output.md", reviewVersion},
	"playbook.task":            {"prompts/playbook/task.md", authoringVersion},
	"playbook.retry":           {"prompts/playbook/retry.md", retryVersion},
	"playbook.spec":            {"prompts/playbook/spec.md", authoringVersion},
	"playbook.html":            {"prompts/playbook/html.md", authoringVersion},
}
