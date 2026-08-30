package workflow

import (
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestRuntimeSystemPromptIncludesSelectedSkillSnapshots(t *testing.T) {
	prompt := buildRuntimeSystemPrompt(runtimePromptInput{
		Phase: PhaseChat,
		Mode:  model.ModeTalk,
		Context: contextengine.ContextPack{Command: model.RunCommand{
			Mode: model.ModeTalk,
			Skills: []model.RunSkill{{
				ID: "story", Name: "演示叙事", Description: "梳理页面叙事。",
				Content: "Always lead with the conclusion.",
			}},
		}},
	})
	for _, expected := range []string{
		`id="skill/story"`,
		`path="skill://story"`,
		"# Active Skill: 演示叙事",
		"Always lead with the conclusion.",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt does not contain %q:\n%s", expected, prompt)
		}
	}
}
