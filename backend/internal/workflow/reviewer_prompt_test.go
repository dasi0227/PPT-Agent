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
	p := &llmtest.FakeProvider{Script: []llm.GenerateResponse{{Content: llm.TextContent(`{"checks":[{"code":"REVIEW_PASS","summary":"The supplied proposal meets the stated requirements."}]}`)}}}
	result, err := (LLMSemanticReviewer{Provider: p}).Review(context.Background(), SemanticReviewInput{ContextBriefing: "PRIVATE_TASK_SENTINEL"})
	if err != nil || !result.Passed() {
		t.Fatalf("review: %+v, %v", result, err)
	}
	req := p.Requests()[0]
	m := prompts.MustLoad("subagent.reviewer.agent")
	if strings.Count(req.Messages[0].Text(), "<prompt_module ") != 1 || !strings.Contains(req.Messages[0].Text(), m.Body) || !strings.Contains(req.Messages[0].Text(), m.Hash) {
		t.Fatal("reviewer policy was not merged/manifested")
	}
	if strings.Contains(req.Messages[0].Text(), "PRIVATE_TASK_SENTINEL") || !strings.Contains(req.Messages[1].Text(), "PRIVATE_TASK_SENTINEL") || strings.Contains(req.Messages[1].Text(), `"rubric"`) || len(req.Tools) != 0 {
		t.Fatal("incorrect review input isolation")
	}
	var manifest struct {
		Modules []map[string]string `json:"modules"`
	}
	if err := json.Unmarshal([]byte(semanticReviewerPromptManifest()), &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Modules) != 1 || manifest.Modules[0]["hash"] != m.Hash || manifest.Modules[0]["path"] != m.Path {
		t.Fatal("persisted manifest does not describe policy")
	}
}
