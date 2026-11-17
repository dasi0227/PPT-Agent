package contextengine

import (
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type ConversationHistoryWindow struct {
	MaxRounds       int // Zero leaves the round count unlimited.
	SoftTokenBudget int // Zero leaves the token count unlimited.
}

// DecisionHistoryWindow is shared by Jev intent, tool pruning and rename checks.
// Ordinary CJK history uses about 13.5 KB; the complete request still needs Fits.
func DecisionHistoryWindow() ConversationHistoryWindow {
	return ConversationHistoryWindow{MaxRounds: 4, SoftTokenBudget: 1500}
}

type ConversationHistory struct {
	Messages []model.ConversationMessage
	Text     string
}

// BuildConversationHistory keeps recent whole rounds, including an unfinished
// latest round. The newest round is retained even if it exceeds the soft budget.
func BuildConversationHistory(rounds []model.ConversationRound, window ConversationHistoryWindow) ConversationHistory {
	var texts []string
	var messages [][]model.ConversationMessage
	for _, round := range rounds {
		var text strings.Builder
		var kept []model.ConversationMessage
		for _, message := range round.Messages {
			if (message.Role != "user" && message.Role != "assistant") || strings.TrimSpace(message.Text) == "" {
				continue
			}
			text.WriteString("<" + message.Role + ">\n" + EscapeCommandContextText(message.Text) + "\n</" + message.Role + ">\n")
			kept = append(kept, message)
		}
		if len(kept) > 0 {
			texts = append(texts, text.String())
			messages = append(messages, kept)
		}
	}
	start := 0
	if window.MaxRounds > 0 && len(texts) > window.MaxRounds {
		start = len(texts) - window.MaxRounds
	}
	wrap := func(start int) string {
		return "<conversation_history>\n" + strings.Join(texts[start:], "") + "</conversation_history>"
	}
	for window.SoftTokenBudget > 0 && start+1 < len(texts) && llm.EstimateTextTokens(wrap(start)) > window.SoftTokenBudget {
		start++
	}
	history := ConversationHistory{Text: wrap(start), Messages: []model.ConversationMessage{}}
	for _, round := range messages[start:] {
		history.Messages = append(history.Messages, round...)
	}
	return history
}
