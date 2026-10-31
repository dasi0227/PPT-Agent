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

func TestReviewerLoadsOnePolicyAndKeepsInputDynamic(t *testing.T) {
	p := &llmtest.FakeProvider{Script: []llm.GenerateResponse{{Content: llm.TextContent("The supplied artifact meets the stated requirements.")}}}
	result, err := (LLMTaskReviewer{Provider: p}).Review(context.Background(), ReviewInput{Material: ReviewMaterial{Demand: "PRIVATE_TASK_SENTINEL"}})
	if err != nil || result.Result != "The supplied artifact meets the stated requirements." {
		t.Fatalf("review: %+v, %v", result, err)
	}
	req := p.Requests()[0]
	shared := prompts.MustLoad("core_agent")
	m := prompts.MustLoad("subagent_reviewer_agent")
	if !strings.Contains(req.Messages[0].Text(), shared.Render()) || !strings.Contains(req.Messages[0].Text(), m.Render()) || strings.Contains(req.Messages[0].Text(), m.Hash) {
		t.Fatal("reviewer policy was not merged/manifested")
	}
	if strings.Contains(req.Messages[0].Text(), `<core_protocols desc="`) || strings.Contains(req.Messages[0].Text(), `<playbook_tool desc="`) {
		t.Fatal("reviewer inherited main create retry/error guidance")
	}
	if strings.Contains(req.Messages[0].Text(), "PRIVATE_TASK_SENTINEL") || !strings.Contains(req.Messages[1].Text(), "PRIVATE_TASK_SENTINEL") || strings.Contains(req.Messages[1].Text(), `"rubric"`) || len(req.Tools) != 0 {
		t.Fatal("incorrect review input isolation")
	}
	want := "<system_prompt>\n" + shared.Render() + "\n" + m.Render() + "\n</system_prompt>"
	if req.Messages[0].Text() != want {
		t.Fatal("reviewer system modules are out of order")
	}
	var manifest struct {
		Modules []map[string]string `json:"modules"`
	}
	if err := json.Unmarshal([]byte(reviewerPromptManifest()), &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Modules) != 2 || manifest.Modules[0]["id"] != "core_agent" || manifest.Modules[0]["hash"] != shared.Hash || manifest.Modules[1]["hash"] != m.Hash || manifest.Modules[1]["path"] != m.Path {
		t.Fatal("persisted manifest does not describe policy")
	}
}
