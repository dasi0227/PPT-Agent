package backend

import "embed"

// PromptFiles contains the production Markdown prompts compiled into the binary.
//
//go:embed prompts
var PromptFiles embed.FS
