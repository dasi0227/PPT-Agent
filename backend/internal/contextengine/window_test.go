package contextengine

import (
	"math"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

func TestPromptEstimatorReturnsFixedSixBucketContract(t *testing.T) {
	snapshot := (PromptEstimator{}).Estimate(PromptEstimateInput{
		System: "policy",
		User: `<runtime_input>
<user_instruction>"make a deck"</user_instruction>
<context_pack>
<run_command>{"mode":"execute"}</run_command>
<project_context>{"title":"deck"}</project_context>
<available_context_refs>[{"id":"ref_1"}]</available_context_refs>
</context_pack>
<runtime_state>{"phase":"executing"}</runtime_state>
</runtime_input>`,
		Messages: []llm.Message{
			{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "read_ppt", Args: map[string]any{"path": "slide.html"}},
					{ID: "call-2", Name: "run_command", Args: map[string]any{"command": "pwd"}},
				},
			},
			{Role: llm.RoleTool, ToolCallID: "call-1", Content: []llm.ContentPart{{Type: "text", Text: "html"}, {Type: "image", ImageRef: "shot"}}},
			{Role: llm.RoleTool, ToolCallID: "call-2", Content: llm.TextContent(`{"stdout":"/tmp","exit_code":0}`)},
		},
		Tools:  []llm.ToolSchema{{Name: "read_ppt", Parameters: map[string]any{"type": "object"}}},
		Max:    65536,
		Factor: 1,
	})

	if snapshot.Total <= 1024 || snapshot.Max != 65536 || snapshot.Ratio <= 0 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	wantDetailCounts := map[ContextBucket]int{
		BucketSystemPrompt: 2,
		BucketRuntime:      3,
		BucketChatHistory:  4,
		BucketReadFile:     3,
		BucketRunCommand:   1,
		BucketOther:        1,
	}
	total := 0
	for _, bucket := range ContextBuckets {
		if _, ok := snapshot.Buckets[bucket]; !ok {
			t.Fatalf("missing bucket %s", bucket)
		}
		if len(snapshot.Details[bucket]) != wantDetailCounts[bucket] {
			t.Fatalf("bucket %s details=%+v", bucket, snapshot.Details[bucket])
		}
		for index, detail := range snapshot.Details[bucket] {
			if detail.Name != ContextWindowDetailNames(bucket)[index] {
				t.Fatalf("bucket %s detail order=%+v", bucket, snapshot.Details[bucket])
			}
		}
		total += snapshot.Buckets[bucket]
	}
	if total != snapshot.Total {
		t.Fatalf("bucket total=%d snapshot total=%d", total, snapshot.Total)
	}
	if snapshot.Buckets[BucketSystemPrompt] == 0 || snapshot.Buckets[BucketRuntime] == 0 ||
		snapshot.Buckets[BucketChatHistory] == 0 || snapshot.Buckets[BucketReadFile] <= 1024 ||
		snapshot.Buckets[BucketRunCommand] == 0 || snapshot.Buckets[BucketOther] == 0 {
		t.Fatalf("bucket classification failed: %+v", snapshot.Buckets)
	}
}

func TestPromptEstimatorSplitsUserTextFromImageAttachments(t *testing.T) {
	snapshot := (PromptEstimator{}).Estimate(PromptEstimateInput{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentPart{
			{Type: "text", Text: "Use this brand image"},
			{Type: "text", Text: `<image_attachment>{"attachment_id":"att_brand","name":"brand.png"}</image_attachment>`},
			{Type: "image", ImageRef: "project:pro_a/attachment:att_brand/original", MIMEType: "image/png"},
		}}},
		Max: 65536, Factor: 1,
	})
	if detailToken(snapshot, BucketReadFile, "read_image") <= imageApproxTokens {
		t.Fatalf("image attachment was not classified as read_image: %+v", snapshot.Details)
	}
	if detailToken(snapshot, BucketChatHistory, "user messages") == 0 {
		t.Fatalf("user text was not preserved: %+v", snapshot.Details)
	}
}

func TestPromptEstimatorSeparatesRuntimeMessagesAndContextSummary(t *testing.T) {
	snapshot := (PromptEstimator{}).Estimate(PromptEstimateInput{Messages: []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("Ordinary assistant text is not a completion signal. Continue with a tool.")},
		{Role: llm.RoleUser, Content: llm.TextContent("<context_summary>finished earlier work</context_summary>")},
		{Role: llm.RoleUser, Content: llm.TextContent("User steering: use dark colors")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("I will update the page.")},
	}})
	for _, tc := range []struct {
		bucket ContextBucket
		name   string
	}{
		{BucketRuntime, "runtime messages"},
		{BucketChatHistory, "context summary"},
		{BucketChatHistory, "user messages"},
		{BucketChatHistory, "assistant messages"},
	} {
		if detailToken(snapshot, tc.bucket, tc.name) == 0 {
			t.Fatalf("missing %s: %+v", tc.name, snapshot.Details)
		}
	}
}

func TestPromptEstimatorClassifiesAttributedResourceSections(t *testing.T) {
	snapshot := (PromptEstimator{}).Estimate(PromptEstimateInput{
		User: `<runtime_input><active_run_skills source="run_snapshot">[{"id":"story"}]</active_run_skills></runtime_input>`,
	})
	if detailToken(snapshot, BucketRuntime, "runtime resources") == 0 {
		t.Fatalf("attributed resource section was not classified: %+v", snapshot.Details)
	}
}

func detailToken(snapshot WindowSnapshot, bucket ContextBucket, name string) int {
	for _, detail := range snapshot.Details[bucket] {
		if detail.Name == name {
			return detail.Tokens
		}
	}
	return 0
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
