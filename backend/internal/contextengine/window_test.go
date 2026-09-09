package contextengine

import (
	"math"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

func TestPromptEstimatorAccountsForBucketsToolsAndImages(t *testing.T) {
	estimator := PromptEstimator{}
	snapshot := estimator.Estimate(PromptEstimateInput{
		System: "policy",
		User: `<runtime_input>
<user_instruction>"make a deck"</user_instruction>
<run_command>{"mode":"execute"}</run_command>
<project_context>{"title":"deck"}</project_context>
<runtime_state>{"phase":"executing"}</runtime_state>
</runtime_input>`,
		Messages: []llm.Message{
			{
				Role:      llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "read_ppt", Args: map[string]any{"path": "slide.html"}}},
			},
			{
				Role: llm.RoleTool, ToolCallID: "call-1",
				Content: []llm.ContentPart{{Type: "text", Text: "html"}, {Type: "image", ImageRef: "shot"}},
			},
		},
		Tools:  []llm.ToolSchema{{Name: "read_ppt", Parameters: map[string]any{"type": "object"}}},
		Max:    65536,
		Factor: 1,
	})
	if snapshot.Total <= 1024 || snapshot.Max != 65536 || snapshot.Ratio <= 0 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	for _, bucket := range ContextBuckets {
		if _, ok := snapshot.Buckets[bucket]; !ok {
			t.Fatalf("missing bucket %s", bucket)
		}
	}
	if snapshot.Buckets[BucketSystemPrompt] == 0 ||
		snapshot.Buckets[BucketReadPPT] <= 1024 ||
		snapshot.Buckets[BucketRunCommand] == 0 ||
		snapshot.Buckets[BucketUserPrompt] == 0 ||
		snapshot.Buckets[BucketOther] == 0 {
		t.Fatalf("bucket classification failed: %+v", snapshot.Buckets)
	}
}

func TestPromptEstimatorSeparatesProjectImageAttachments(t *testing.T) {
	snapshot := (PromptEstimator{}).Estimate(PromptEstimateInput{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentPart{
			{Type: "text", Text: `<image_attachment>{"attachment_id":"att_brand","name":"brand.png"}</image_attachment>`},
			{Type: "image", ImageRef: "project:pro_a/attachment:att_brand/original", MIMEType: "image/png"},
		}}},
		Max: 65536, Factor: 1,
	})
	if snapshot.Buckets[BucketUploadedFile] <= imageApproxTokens || snapshot.Buckets[BucketUserPrompt] == 0 {
		t.Fatalf("attachment bucket classification failed: %+v", snapshot.Buckets)
	}
	if len(snapshot.Details[BucketUploadedFile]) != 2 || snapshot.Details[BucketUploadedFile][0].Name != "brand.png" {
		t.Fatalf("attachment details missing: %+v", snapshot.Details[BucketUploadedFile])
	}
}

func TestCalibrationStoreUsesBoundedEMA(t *testing.T) {
	store := NewCalibrationStore()
	if got := store.Factor("thread"); got != 1 {
		t.Fatalf("initial factor=%f", got)
	}
	first := store.Observe("thread", 100, 200)
	if math.Abs(first-1.2) > 0.0001 {
		t.Fatalf("first factor=%f", first)
	}
	second := store.Observe("thread", 100, 1000)
	if second <= first || second > 2 {
		t.Fatalf("bounded EMA factor=%f", second)
	}
}
