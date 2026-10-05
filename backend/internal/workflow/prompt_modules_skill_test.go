package workflow

import (
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestRuntimeSystemPromptExcludesSelectedSkillSnapshots(t *testing.T) {
	prompt := buildRuntimeSystemPrompt(runtimePromptInput{
		Phase: PhaseChat,
		Mode:  model.ModeChat,
		Context: contextengine.ContextPack{Command: model.RunCommand{
			Mode: model.ModeChat,
			Skills: []model.RunSkill{{
				ID: "story", Name: "演示叙事", Description: "梳理页面叙事。",
				Content: "Always lead with the conclusion.",
			}},
		}},
	})
	for _, forbidden := range []string{"skill/story", "演示叙事", "Always lead with the conclusion."} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("stable system prompt contains dynamic skill %q:\n%s", forbidden, prompt)
		}
	}
	request := prepareAgentRequest(AgentRequest{
		Context:      testPack(model.ModeChat, model.ScopeAllPages, false, "skill context"),
		ActiveSkills: []model.RunSkill{{ID: "story", Name: "演示叙事", Content: "Always lead with the conclusion."}},
	})
	found := false
	for _, message := range request.Messages {
		found = found || strings.Contains(message.Text(), "Always lead with the conclusion.")
	}
	if !found {
		t.Fatal("dynamic skill context omitted snapshot content")
	}
}
