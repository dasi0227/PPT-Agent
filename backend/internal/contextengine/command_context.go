package contextengine

import "strings"

var commandContextDelimiters = func() *strings.Replacer {
	pairs := []string{}
	for _, tag := range []string{"project_context", "manifest", "design", "outline", "slide_summaries", "current_plan", "current_title", "classification_rules", "selection_rules", "renaming_rules", "conversation_history", "current_user_input", "user", "assistant", "user_instruction", "last_revision", "revision_feedback"} {
		pairs = append(pairs, "<"+tag+">", "&lt;"+tag+"&gt;", "</"+tag+">", "&lt;/"+tag+"&gt;")
	}
	return strings.NewReplacer(pairs...)
}()

// EscapeCommandContextText protects shared project, history and revision blocks.
// Only exact structural delimiters are escaped; other text/code stays literal.
func EscapeCommandContextText(text string) string { return commandContextDelimiters.Replace(text) }
