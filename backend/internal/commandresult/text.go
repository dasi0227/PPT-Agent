// Package commandresult defines the result contract for single-call text commands.
package commandresult

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

const MaxTitleRunes = 48

type Text struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func Schema(name, description, titleDescription, contentDescription string, maxContentRunes int) llm.ToolSchema {
	content := map[string]any{"type": "string", "minLength": 1, "description": contentDescription}
	if maxContentRunes > 0 {
		content["maxLength"] = maxContentRunes
	}
	return llm.ToolSchema{
		Name: name, Description: description,
		Parameters: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []string{"title", "content"},
			"properties": map[string]any{
				"title":   map[string]any{"type": "string", "minLength": 1, "maxLength": MaxTitleRunes, "description": titleDescription},
				"content": content,
			},
		},
	}
}

func Parse(response llm.GenerateResponse, name string, maxContentRunes int) (Text, error) {
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != name {
		return Text{}, fmt.Errorf("model must call %s exactly once", name)
	}
	for _, part := range response.Content {
		if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
			return Text{}, fmt.Errorf("%s must return its result only through the tool", name)
		}
	}
	args := response.ToolCalls[0].Args
	if len(args) != 2 {
		return Text{}, fmt.Errorf("%s requires only title and content", name)
	}
	rawTitle, titleOK := args["title"].(string)
	rawContent, contentOK := args["content"].(string)
	title, content := strings.TrimSpace(rawTitle), strings.TrimSpace(rawContent)
	if !titleOK || title == "" || utf8.RuneCountInString(title) > MaxTitleRunes || strings.ContainsAny(title, "<>\r\n\t") {
		return Text{}, fmt.Errorf("%s title must be short single-line plain text", name)
	}
	for _, char := range rawTitle {
		if unicode.IsControl(char) {
			return Text{}, fmt.Errorf("%s title contains control characters", name)
		}
	}
	for _, prefix := range []string{"#", "- ", "* ", "+ ", ">", "```"} {
		if strings.HasPrefix(title, prefix) {
			return Text{}, fmt.Errorf("%s title must not contain Markdown markers", name)
		}
	}
	if !contentOK || content == "" || (maxContentRunes > 0 && utf8.RuneCountInString(content) > maxContentRunes) {
		return Text{}, fmt.Errorf("%s content is empty or too long", name)
	}
	return Text{Title: title, Content: content}, nil
}
