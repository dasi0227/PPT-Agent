package service

import "strings"

func buildPolishInput(instruction, lastRevision, feedback string) string {
	// Escape only exact block delimiters; preserve the draft's other text.
	literalTags := strings.NewReplacer(
		"<user_draft>", "&lt;user_draft&gt;", "</user_draft>", "&lt;/user_draft&gt;",
		"<last_revision>", "&lt;last_revision&gt;", "</last_revision>", "&lt;/last_revision&gt;",
		"<revision_feedback>", "&lt;revision_feedback&gt;", "</revision_feedback>", "&lt;/revision_feedback&gt;",
	)
	input := "<user_draft>\n" + literalTags.Replace(instruction) + "\n</user_draft>"
	if strings.TrimSpace(lastRevision) != "" {
		input += "\n\n<last_revision>\n" + literalTags.Replace(lastRevision) +
			"\n</last_revision>\n<revision_feedback>\n" + literalTags.Replace(feedback) + "\n</revision_feedback>"
	}
	return input
}
