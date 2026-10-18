package backend

import "embed"

// PromptFiles contains production prompts.
//
//go:embed prompts/command prompts/core prompts/mode prompts/playbook prompts/rule prompts/subagent
var PromptFiles embed.FS
