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
	p := &llmtest.FakeProvider{Script: []llm.GenerateResponse{{ToolCalls: []llm.ToolCall{{ID: "submit", Name: "submit_review", Args: map[string]any{"type": "approve", "reasons": []any{"The supplied artifact meets the stated requirements."}}}}}}}
	result, err := (LLMTaskReviewer{Provider: p}).Review(context.Background(), ReviewInput{Material: ReviewMaterial{Demand: "PRIVATE_TASK_SENTINEL"}})
	if err != nil || result.Type != "approve" {
		t.Fatalf("review: %+v, %v", result, err)
	}
	req := p.Requests()[0]
	m := prompts.MustLoad("subagent.reviewer.agent")
	if !strings.Contains(req.Messages[0].Text(), prompts.MustLoad("core.quality").Body) || !strings.Contains(req.Messages[0].Text(), m.Body) || strings.Contains(req.Messages[0].Text(), m.Hash) {
		t.Fatal("reviewer policy was not merged/manifested")
	}
	if strings.Contains(req.Messages[0].Text(), "PRIVATE_TASK_SENTINEL") || !strings.Contains(req.Messages[1].Text(), "PRIVATE_TASK_SENTINEL") || strings.Contains(req.Messages[1].Text(), `"rubric"`) || len(req.Tools) != 1 || req.Tools[0].Name != "submit_review" {
		t.Fatal("incorrect review input isolation")
	}
	var manifest struct {
		Modules []map[string]string `json:"modules"`
	}
	if err := json.Unmarshal([]byte(reviewerPromptManifest()), &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Modules) != 2 || manifest.Modules[1]["hash"] != m.Hash || manifest.Modules[1]["path"] != m.Path {
		t.Fatal("persisted manifest does not describe policy")
	}
}
