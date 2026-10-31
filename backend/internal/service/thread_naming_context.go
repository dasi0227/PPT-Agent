package service

import (
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/decision"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/prompt"
)

type renameInput struct {
	CurrentTitle    string
	Activity        []model.ConversationMessage
	history         string
}

func newRenameInput(title string, source model.ThreadConversationContext) renameInput {
	history := contextengine.BuildConversationHistory(source.Rounds, contextengine.ConversationHistoryWindow{
		MaxRounds: 10, SoftTokenBudget: 10000,
	})
	return renameInput{CurrentTitle: title, Activity: history.Messages, history: history.Text}
}

func buildRenameDecisionRequest(title string, source model.ThreadConversationContext) (decision.Request, prompt.Module, error) {
	if source.Input == nil {
		return decision.Request{}, prompt.Module{}, errors.New("rename input is not durably accepted")
	}
	rules, err := prompt.Load("jev.rename")
	if err != nil {
		return decision.Request{}, prompt.Module{}, err
	}
	history := contextengine.BuildConversationHistory(source.Rounds, contextengine.DecisionHistoryWindow())
	escape := contextengine.EscapeCommandContextText
	state := "<renaming_rules>\n" + rules.Body + "\n</renaming_rules>\n\n<current_title>\n" +
		escape(title) + "\n</current_title>\n\n" + history.Text + "\n\n<current_user_input>\n" +
		escape(source.Input.Text) + "\n</current_user_input>"
	return decision.Request{State: state, Questions: map[string]decision.Question{
		"rename": decision.NoulQuestion{Instructions: "Should current_title be changed according to renaming_rules?"},
	}}, rules, nil
}

func buildRenameInput(input renameInput) string {
	return "<current_title>\n" + contextengine.EscapeCommandContextText(input.CurrentTitle) + "\n</current_title>\n\n" + input.history +
		"\n\n<user_instruction>\nDecide whether to keep or replace the current conversation title based on the supplied context.\n</user_instruction>"
}
