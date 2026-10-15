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
	p := &llmtest.FakeProvider{Script: []llm.GenerateResponse{{Content: llm.TextContent(`{"decision":"approve","reasons":["The supplied artifact meets the stated requirements."]}`)}}}
	result, err := (LLMTaskReviewer{Provider: p}).Review(context.Background(), ReviewInput{Material: ReviewMaterial{Demand: "PRIVATE_TASK_SENTINEL"}})
	if err != nil || result.Decision != "approve" {
		t.Fatalf("review: %+v, %v", result, err)
	}
	req := p.Requests()[0]
	m := prompts.MustLoad("subagent.reviewer.agent")
	if !strings.Contains(req.Messages[0].Text(), prompts.MustLoad("core.quality").SystemPrompt()) || !strings.Contains(req.Messages[0].Text(), m.SystemPrompt()) || strings.Contains(req.Messages[0].Text(), m.Hash) {
		t.Fatal("reviewer policy was not merged/manifested")
	}
	if strings.Contains(req.Messages[0].Text(), "## Protocol「IMPORTANT」") || strings.Contains(req.Messages[0].Text(), "## Procedure") {
		t.Fatal("reviewer inherited main execute retry/error guidance")
	}
	if strings.Contains(req.Messages[0].Text(), "PRIVATE_TASK_SENTINEL") || !strings.Contains(req.Messages[1].Text(), "PRIVATE_TASK_SENTINEL") || strings.Contains(req.Messages[1].Text(), `"rubric"`) || len(req.Tools) != 0 {
		t.Fatal("incorrect review input isolation")
	}
	want := m.SystemPrompt() + "\n\n" + prompts.MustLoad("core.quality").SystemPrompt() + "\n\n" + prompts.MustLoad("subagent.reviewer.output").SystemPrompt()
	if req.Messages[0].Text() != want {
		t.Fatal("reviewer system modules are out of order")
	}
	var manifest struct {
		Modules []map[string]string `json:"modules"`
	}
	if err := json.Unmarshal([]byte(reviewerPromptManifest()), &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Modules) != 3 || manifest.Modules[0]["hash"] != m.Hash || manifest.Modules[0]["path"] != m.Path || manifest.Modules[1]["id"] != "core.quality" || manifest.Modules[2]["id"] != "subagent.reviewer.output" {
		t.Fatal("persisted manifest does not describe policy")
	}
}
