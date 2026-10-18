package service

import "strings"

func buildPolishInput(instruction, polishContent, feedback string) string {
	// Escape only exact block delimiters; preserve the draft's other text.
	literalTags := strings.NewReplacer(
		"<user_draft>", "&lt;user_draft&gt;", "</user_draft>", "&lt;/user_draft&gt;",
		"<polish_revision>", "&lt;polish_revision&gt;", "</polish_revision>", "&lt;/polish_revision&gt;",
		"<polish_content>", "&lt;polish_content&gt;", "</polish_content>", "&lt;/polish_content&gt;",
		"<revision_feedback>", "&lt;revision_feedback&gt;", "</revision_feedback>", "&lt;/revision_feedback&gt;",
	)
	input := "<user_draft>\n" + literalTags.Replace(instruction) + "\n</user_draft>"
	if strings.TrimSpace(feedback) != "" && strings.TrimSpace(polishContent) != "" {
		input += "\n\n<polish_revision>\n  <polish_content>\n" + literalTags.Replace(polishContent) +
			"\n  </polish_content>\n  <revision_feedback>\n" + literalTags.Replace(feedback) +
			"\n  </revision_feedback>\n</polish_revision>"
	}
	return input
}
