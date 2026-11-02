package backend

import "embed"

// PromptFiles contains production prompts.
//
//go:embed prompts/addition prompts/command prompts/core prompts/decision prompts/jev prompts/mode prompts/playbook prompts/rule prompts/subagent prompts/tool
var PromptFiles embed.FS

// ChatReplyPrompt is the complete system message for a standalone chat reply.
//
//go:embed prompts/other/reply.md
var ChatReplyPrompt string
