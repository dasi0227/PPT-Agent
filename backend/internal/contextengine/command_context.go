package contextengine

import (
	"html"
	"regexp"
	"strings"
)

var commandPromptTags = regexp.MustCompile(`</?(?:system_prompt|fallible_point|(?:command|jev|decision)_[a-z0-9_]+)(?:\s[^<>]*)?\s*/?>`)

var commandContextDelimiters = func() *strings.Replacer {
	pairs := []string{}
	for _, tag := range []string{"project_context", "meta", "constraints", "spec", "html", "outline", "slide_summaries", "current_plan", "current_title", "conversation_history", "current_user_input", "user", "assistant", "user_instruction", "last_revision", "revision_feedback"} {
		pairs = append(pairs, "<"+tag+">", "&lt;"+tag+"&gt;", "</"+tag+">", "&lt;/"+tag+"&gt;")
	}
	return strings.NewReplacer(pairs...)
}()

// EscapeCommandContextText protects shared project, history and revision blocks.
// Prompt tags may carry metadata; other text/code stays literal except for exact data delimiters.
func EscapeCommandContextText(text string) string {
	return commandContextDelimiters.Replace(commandPromptTags.ReplaceAllStringFunc(text, html.EscapeString))
}
