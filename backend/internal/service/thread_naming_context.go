package service

import (
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type renameInput struct {
	CurrentTitle string                       `json:"current_title"`
	Activity     []model.ThreadNamingActivity `json:"activity"`
}

func newRenameInput(title string, source model.ThreadRenameContextSource) renameInput {
	value := renameInput{CurrentTitle: title, Activity: []model.ThreadNamingActivity{}}
	for _, activity := range source.Activity {
		if activity.Role != "user" && activity.Role != "assistant" {
			continue
		}
		if strings.TrimSpace(activity.Text) != "" {
			value.Activity = append(value.Activity, activity)
		}
	}
	return value
}

func buildRenameInput(input renameInput) string {
	// Escape only block delimiters; preserve all other conversation text.
	literalTags := strings.NewReplacer(
		"<current_title>", "&lt;current_title&gt;", "</current_title>", "&lt;/current_title&gt;",
		"<conversation_history>", "&lt;conversation_history&gt;", "</conversation_history>", "&lt;/conversation_history&gt;",
		"<user>", "&lt;user&gt;", "</user>", "&lt;/user&gt;",
		"<assistant>", "&lt;assistant&gt;", "</assistant>", "&lt;/assistant&gt;",
	)
	var text strings.Builder
	text.WriteString("<current_title>\n" + literalTags.Replace(input.CurrentTitle) + "\n</current_title>\n\n<conversation_history>\n")
	for _, activity := range input.Activity {
		text.WriteString("<" + activity.Role + ">\n" + literalTags.Replace(activity.Text) + "\n</" + activity.Role + ">\n")
	}
	text.WriteString("</conversation_history>")
	return text.String()
}
