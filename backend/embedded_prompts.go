package backend

import "embed"

// PromptFiles contains production prompts.
//
//go:embed prompts/command prompts/core prompts/decision prompts/jev prompts/mode prompts/playbook prompts/rule prompts/subagent prompts/tool
var PromptFiles embed.FS
