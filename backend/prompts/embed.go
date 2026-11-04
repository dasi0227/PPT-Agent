package prompts

import "embed"

// PromptFiles contains production prompts.
//
//go:embed command core jev mode playbook reminder rule subagent tool
var PromptFiles embed.FS

// ChatReplyPrompt is the complete system message for a standalone chat reply.
//
//go:embed other/reply.md
var ChatReplyPrompt string
