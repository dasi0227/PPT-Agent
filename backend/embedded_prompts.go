package backend

import "embed"

// PromptFiles contains the production Markdown prompts compiled into the binary.
//
//go:embed prompts/command prompts/core prompts/mode prompts/playbook prompts/subagent
var PromptFiles embed.FS
