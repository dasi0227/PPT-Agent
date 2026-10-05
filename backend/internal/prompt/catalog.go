package prompt

// Version identifies this coordinated prompt release; Hash identifies exact content.
const Version = "2026-09-23.v2"

const singleCallVersion = "2026-09-29.v1"
const conversationVersion = "2026-09-29.v2"
const reviewVersion = "2026-09-29.v3"

type entry struct{ Path, Version string }

var catalog = map[string]entry{
	"command.commit":                 {"prompts/command/commit.md", singleCallVersion},
	"command.compact":                {"prompts/command/compact.md", Version},
	"command.handoff":                {"prompts/command/handoff.md", Version},
	"command.polish":                 {"prompts/command/polish.md", singleCallVersion},
	"command.rename":                 {"prompts/command/rename.md", singleCallVersion},
	"core.agent":                     {"prompts/core/agent.md", conversationVersion},
	"core.html":                      {"prompts/core/html.md", Version},
	"core.output":                    {"prompts/core/output.md", Version},
	"core.quality":                   {"prompts/core/quality.md", conversationVersion},
	"core.reference":                 {"prompts/core/reference.md", conversationVersion},
	"mode.chat":                      {"prompts/mode/chat.md", conversationVersion},
	"mode.execute":                   {"prompts/mode/execute.md", reviewVersion},
	"mode.grill":                     {"prompts/mode/grill.md", conversationVersion},
	"mode.plan":                      {"prompts/mode/plan.md", Version},
	"playbook.deck":                  {"prompts/playbook/deck.md", Version},
	"playbook.slide":                 {"prompts/playbook/slide.md", Version},
	"playbook.spec":                  {"prompts/playbook/spec.md", Version},
	"runtime.execution":              {"prompts/runtime/execution.md", conversationVersion},
	"runtime.context":                {"prompts/runtime/context.md", Version},
	"runtime.completion":             {"prompts/runtime/completion.md", conversationVersion},
	"runtime.next-input-suggestions": {"prompts/runtime/next-input-suggestions.md", conversationVersion},
	"runtime.recovery":               {"prompts/runtime/recovery.md", Version},
	"subagent.reviewer.agent":        {"prompts/subagent/reviewer/agent.md", reviewVersion},
}
