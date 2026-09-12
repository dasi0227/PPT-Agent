package prompt

// Version identifies this coordinated prompt release; Hash identifies exact content.
const Version = "2026-09-12.v5"

type entry struct{ Path, Version string }

var catalog = map[string]entry{
	"command.commit":          {"prompts/command/commit.md", Version},
	"command.compact":         {"prompts/command/compact.md", Version},
	"command.handoff":         {"prompts/command/handoff.md", Version},
	"command.kickoff":         {"prompts/command/kickoff.md", Version},
	"command.polish":          {"prompts/command/polish.md", Version},
	"core.agent":              {"prompts/core/agent.md", Version},
	"core.html":               {"prompts/core/html.md", Version},
	"core.output":             {"prompts/core/output.md", Version},
	"core.quality":            {"prompts/core/quality.md", Version},
	"core.reference":          {"prompts/core/reference.md", Version},
	"core.structure":          {"prompts/core/structure.md", Version},
	"mode.chat":               {"prompts/mode/chat.md", Version},
	"mode.execute":            {"prompts/mode/execute.md", Version},
	"mode.grill":              {"prompts/mode/grill.md", Version},
	"mode.plan":               {"prompts/mode/plan.md", Version},
	"playbook.deck":           {"prompts/playbook/deck.md", Version},
	"playbook.slide":          {"prompts/playbook/slide.md", Version},
	"playbook.spec":           {"prompts/playbook/spec.md", Version},
	"runtime.completion":      {"prompts/runtime/completion.md", Version},
	"runtime.recovery":        {"prompts/runtime/recovery.md", Version},
	"subagent.reviewer.agent": {"prompts/subagent/reviewer/agent.md", Version},
}
