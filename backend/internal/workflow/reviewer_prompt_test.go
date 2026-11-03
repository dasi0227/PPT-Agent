package workflow

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
)

func TestReviewerLoadsSharedRulesAndKeepsInputDynamic(t *testing.T) {
	p := &llmtest.FakeProvider{Script: []llm.GenerateResponse{{Content: llm.TextContent("The supplied artifact meets the stated requirements.")}}}
	result, err := (LLMTaskReviewer{Provider: p}).Review(context.Background(), ReviewInput{Material: ReviewMaterial{Demand: "PRIVATE_TASK_SENTINEL"}})
	if err != nil || result.Result != "The supplied artifact meets the stated requirements." {
		t.Fatalf("review: %+v, %v", result, err)
	}
	req := p.Requests()[0]
	m := prompts.MustLoad("reviewer_agent")
	if !strings.Contains(req.Messages[0].Text(), m.Render()) || strings.Contains(req.Messages[0].Text(), m.Hash) {
		t.Fatal("reviewer policy was not loaded")
	}
	if strings.Contains(req.Messages[0].Text(), `<core_agent desc="`) || strings.Contains(req.Messages[0].Text(), `<core_contracts desc="`) || strings.Contains(req.Messages[0].Text(), `<core_protocols desc="`) || strings.Contains(req.Messages[0].Text(), `<playbook_tool desc="`) {
		t.Fatal("reviewer inherited main agent guidance")
	}
	if strings.Contains(req.Messages[0].Text(), "PRIVATE_TASK_SENTINEL") || !strings.Contains(req.Messages[1].Text(), "PRIVATE_TASK_SENTINEL") || strings.Contains(req.Messages[1].Text(), `"rubric"`) || len(req.Tools) != 0 {
		t.Fatal("incorrect review input isolation")
	}
	modules := []prompts.Module{m, prompts.MustLoad("rule_authoring"), prompts.MustLoad("rule_styling")}
	want := "<system_prompt>\n" + modules[0].Render() + "\n" + modules[1].Render() + "\n" + modules[2].Render() + "\n</system_prompt>"
	if req.Messages[0].Text() != want {
		t.Fatal("reviewer system modules are out of order")
	}
	var manifest struct {
		Modules []map[string]string `json:"modules"`
	}
	if err := json.Unmarshal([]byte(reviewerPromptManifest()), &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Modules) != len(modules) {
		t.Fatal("persisted manifest does not describe policy")
	}
	for i, module := range modules {
		if manifest.Modules[i]["id"] != module.ID || manifest.Modules[i]["hash"] != module.Hash || manifest.Modules[i]["path"] != module.Path {
			t.Fatal("persisted manifest does not describe shared rules")
		}
	}
}
