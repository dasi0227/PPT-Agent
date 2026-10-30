package service

import (
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

const suggestHistorySoftTokens = 6000

func buildSuggestInput(snapshot contextengine.ProjectResourceSnapshot, source model.ThreadConversationContext) string {
	escape := contextengine.EscapeCommandContextText
	blocks := []string{contextengine.CompileProjectResourceContext(snapshot)}
	if plan := source.Plan; plan != nil {
		var text strings.Builder
		fmt.Fprintf(&text, "标题：%s\n状态：%s\n步骤：\n", escape(plan.Title), escape(plan.Status))
		for i, step := range plan.Steps {
			fmt.Fprintf(&text, "%d. [%s] %s\n", i+1, escape(step.Status), escape(step.Title))
		}
		blocks = append(blocks, "<current_plan>\n"+text.String()+"</current_plan>")
	}
	history := contextengine.BuildConversationHistory(source.Rounds, contextengine.ConversationHistoryWindow{
		MaxRounds: 5, SoftTokenBudget: suggestHistorySoftTokens,
	})
	blocks = append(blocks, history.Text)
	blocks = append(blocks, "<user_instruction>\nSuggest useful requests the user could send next based on the supplied context.\n</user_instruction>")
	return strings.Join(blocks, "\n\n")
}
