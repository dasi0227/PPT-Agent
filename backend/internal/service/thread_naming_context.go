package service

import (
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type renameInput struct {
	CurrentTitle string                      `json:"current_title"`
	Activity     []model.ConversationMessage `json:"activity"`
	history      string
}

func newRenameInput(title string, source model.ThreadConversationContext) renameInput {
	history := contextengine.BuildConversationHistory(source.Rounds, contextengine.ConversationHistoryWindow{
		MaxRounds: 10, SoftTokenBudget: 10000,
	})
	return renameInput{CurrentTitle: title, Activity: history.Messages, history: history.Text}
}

func buildRenameInput(input renameInput) string {
	return "<current_title>\n" + contextengine.EscapeCommandContextText(input.CurrentTitle) + "\n</current_title>\n\n" + input.history +
		"\n\n<user_instruction>\nDecide whether to keep or replace the current conversation title based on the supplied context.\n</user_instruction>"
}
