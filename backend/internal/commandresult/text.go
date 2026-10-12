// Package commandresult defines the result contract for tool-submitted text commands.
package commandresult

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/tooldefinition"
)

const MaxTitleRunes = tooldefinition.MaxTextTitleRunes

type Text struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func Parse(response llm.GenerateResponse, name string, maxContentRunes int) (Text, error) {
	if err := llm.ValidateSubmissionEnvelope(response, name, true); err != nil {
		return Text{}, err
	}
	args := response.ToolCalls[0].Args
	if len(args) != 2 {
		return Text{}, llm.SubmissionFailure("INVALID_FIELDS", "/", "Supply only title and content, both required strings.")
	}
	rawTitle, titleOK := args["title"].(string)
	rawContent, contentOK := args["content"].(string)
	title, content := strings.TrimSpace(rawTitle), strings.TrimSpace(rawContent)
	if !titleOK || title == "" || utf8.RuneCountInString(title) > MaxTitleRunes || strings.ContainsAny(title, "\r\n\t") {
		return Text{}, llm.SubmissionFailure("INVALID_TITLE", "/title", "title must be non-empty single-line plain text, at most 48 characters, without control characters.")
	}
	for _, char := range rawTitle {
		if unicode.IsControl(char) {
			return Text{}, llm.SubmissionFailure("INVALID_TITLE", "/title", "title must not contain control characters.")
		}
	}
	for _, prefix := range []string{"#", "- ", "* ", "+ ", ">", "```"} {
		if strings.HasPrefix(title, prefix) {
			return Text{}, llm.SubmissionFailure("INVALID_TITLE", "/title", "title must not contain Markdown markers.")
		}
	}
	if !contentOK || content == "" || (maxContentRunes > 0 && utf8.RuneCountInString(content) > maxContentRunes) {
		return Text{}, llm.SubmissionFailure("INVALID_CONTENT", "/content", "content must be a non-empty string within the tool's declared length limit.")
	}
	return Text{Title: title, Content: content}, nil
}
