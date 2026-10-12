package backend

import "embed"

// PromptFiles contains production prompts and tool documentation.
//
//go:embed prompts/command prompts/core prompts/mode prompts/playbook prompts/subagent prompts/tools.json
var PromptFiles embed.FS
