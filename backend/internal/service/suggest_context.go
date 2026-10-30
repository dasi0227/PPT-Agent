package service

import (
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

const suggestHistorySoftTokens = 6000

func buildSuggestInput(outline spec.Outline, source model.ThreadSuggestContextSource) string {
	// Only these delimiters belong to the projection; other source text stays literal.
	literalTags := strings.NewReplacer(
		"<outline>", "&lt;outline&gt;", "</outline>", "&lt;/outline&gt;",
		"<current_plan>", "&lt;current_plan&gt;", "</current_plan>", "&lt;/current_plan&gt;",
		"<conversation_history>", "&lt;conversation_history&gt;", "</conversation_history>", "&lt;/conversation_history&gt;",
		"<user>", "&lt;user&gt;", "</user>", "&lt;/user&gt;",
		"<assistant>", "&lt;assistant&gt;", "</assistant>", "&lt;/assistant&gt;",
	)
	blocks := []string{}
	if len(outline.Sections) > 0 {
		var text strings.Builder
		page := 0
		for i, section := range outline.Sections {
			fmt.Fprintf(&text, "第 %d 章：%s\n", i+1, literalTags.Replace(section.Title))
			for _, slide := range section.Slides {
				page++
				fmt.Fprintf(&text, "  第 %d 页：%s\n", page, literalTags.Replace(slide.Title))
			}
			for j, subsection := range section.Subsections {
				fmt.Fprintf(&text, "  第 %d 节：%s\n", j+1, literalTags.Replace(subsection.Title))
				for _, slide := range subsection.Slides {
					page++
					fmt.Fprintf(&text, "    第 %d 页：%s\n", page, literalTags.Replace(slide.Title))
				}
			}
		}
		blocks = append(blocks, "<outline>\n"+text.String()+"</outline>")
	}
	if plan := source.Plan; plan != nil {
		var text strings.Builder
		fmt.Fprintf(&text, "标题：%s\n状态：%s\n步骤：\n", literalTags.Replace(plan.Title), literalTags.Replace(plan.Status))
		for i, step := range plan.Steps {
			fmt.Fprintf(&text, "%d. [%s] %s\n", i+1, literalTags.Replace(step.Status), literalTags.Replace(step.Title))
		}
		blocks = append(blocks, "<current_plan>\n"+text.String()+"</current_plan>")
	}
	// Count the actual escaped history, including role tags. Outline and plan
	// never consume the history budget, and the newest complete round stays whole.
	rounds := make([]string, len(source.Rounds))
	for i, round := range source.Rounds {
		var text strings.Builder
		for _, message := range round.Messages {
			if (message.Role == "user" || message.Role == "assistant") && strings.TrimSpace(message.Text) != "" {
				text.WriteString("<" + message.Role + ">\n" + literalTags.Replace(message.Text) + "\n</" + message.Role + ">\n")
			}
		}
		rounds[i] = text.String()
	}
	start := 0
	for start+1 < len(rounds) && llm.EstimateTextTokens("<conversation_history>\n"+strings.Join(rounds[start:], "")+"</conversation_history>") > suggestHistorySoftTokens {
		start++
	}
	blocks = append(blocks, "<conversation_history>\n"+strings.Join(rounds[start:], "")+"</conversation_history>")
	return strings.Join(blocks, "\n\n")
}
