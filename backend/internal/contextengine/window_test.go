package contextengine

import (
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

func TestPromptEstimatorReturnsFiveBucketContract(t *testing.T) {
	snapshot := (PromptEstimator{}).Estimate(PromptEstimateInput{
		System: "policy",

		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: llm.TextContent("make a deck")},
			{Role: llm.RoleUser, Content: llm.TextContent(`<runtime_context id="run_state" desc="Current run.">{"run_mode":"execute"}</runtime_context>`), Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "context", Key: "run_state"}},
			{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "read_resource", Args: map[string]any{"path": "slide.html"}},
					{ID: "call-2", Name: "run_command", Args: map[string]any{"command": "pwd"}},
				},
			},
			{Role: llm.RoleTool, ToolCallID: "call-1", Content: []llm.ContentPart{{Type: "text", Text: "html"}, {Type: "image", ImageRef: "shot"}}},
			{Role: llm.RoleTool, ToolCallID: "call-2", Content: llm.TextContent(`{"stdout":"/tmp","exit_code":0}`)},
		},
		Tools: []llm.ToolSchema{{Name: "read_resource", Parameters: map[string]any{"type": "object"}}},
		Max:   65536,
	})

	if snapshot.Total <= 1024 || snapshot.Max != 65536 || snapshot.Ratio <= 0 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	wantDetailCounts := map[ContextBucket]int{
		BucketSystemPrompt: 2,
		BucketRuntime:      2,
		BucketChatHistory:  3,
		BucketReadFile:     2,
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
		snapshot.Buckets[BucketOther] == 0 {
		t.Fatalf("bucket classification failed: %+v", snapshot.Buckets)
	}
}

func TestPromptEstimatorSplitsUserTextFromImageAttachments(t *testing.T) {
	for name, metadata := range map[string]*llm.MessageMetadata{
		"ordinary input":            nil,
		"projected current request": {Origin: "runtime", Kind: "user_request", RunID: "run"},
	} {
		t.Run(name, func(t *testing.T) {
			snapshot := (PromptEstimator{}).Estimate(PromptEstimateInput{
				Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentPart{
					{Type: "text", Text: "Use this brand image"},
					{Type: "text", Text: `<image_attachment>{"attachment_id":"att_brand","name":"brand.png"}</image_attachment>`},
					{Type: "image", ImageRef: "project:pro_a/attachment:att_brand/original", MIMEType: "image/png"},
				}, Metadata: metadata}},
				Max: 65536,
			})
			if detailToken(snapshot, BucketReadFile, "read_image") <= imageApproxTokens {
				t.Fatalf("image attachment was not classified as read_image: %+v", snapshot.Details)
			}
			if detailToken(snapshot, BucketChatHistory, "user messages") == 0 {
				t.Fatalf("user text was not preserved: %+v", snapshot.Details)
			}
			if detailToken(snapshot, BucketRuntime, "runtime messages") != 0 {
				t.Fatal("user request was misclassified as runtime state")
			}
		})
	}
}

func TestPromptEstimatorIncludesSummaryInRuntimeMessages(t *testing.T) {
	snapshot := (PromptEstimator{}).Estimate(PromptEstimateInput{Messages: []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("Ordinary assistant text is not a completion signal. Continue with a tool."), Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "guidance"}},
		{Role: llm.RoleUser, Content: llm.TextContent("<context_summary>finished earlier work</context_summary>"), Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "summary"}},
		{Role: llm.RoleUser, Content: llm.TextContent("User steering: use dark colors")},
		{Role: llm.RoleAssistant, Content: llm.TextContent("I will update the page.")},
	}})
	for _, tc := range []struct {
		bucket ContextBucket
		name   string
	}{
		{BucketRuntime, "runtime messages"},
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
		Messages: []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent(`{"skill/story":"full skill"}`), Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "context", Key: "active_skills"}}},
	})
	if detailToken(snapshot, BucketRuntime, "runtime context") == 0 {
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

func TestPromptEstimatorKeepsRenderPixelsInReadImagesAcrossCompaction(t *testing.T) {
	image := llm.ContentPart{Type: "image", ImageRef: "project:pro_a/render:shot"}
	before := (PromptEstimator{}).Estimate(PromptEstimateInput{Messages: []llm.Message{
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "render", Name: "render_slide"}}},
		{Role: llm.RoleTool, ToolCallID: "render", Content: append(llm.TextContent("render diagnostics"), image)},
	}})
	after := (PromptEstimator{}).Estimate(PromptEstimateInput{Messages: []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent("summary"), Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "summary"}},
		{Role: llm.RoleUser, Content: []llm.ContentPart{image}, Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "run_image"}},
	}})
	if detailToken(before, BucketReadFile, "read_image") != imageApproxTokens || detailToken(after, BucketReadFile, "read_image") != imageApproxTokens {
		t.Fatal("render pixels changed attribution after compaction")
	}
	if detailToken(before, BucketChatHistory, "tools execution") <= EstimateTextTokens("render diagnostics") {
		t.Fatal("render call or diagnostics were lost")
	}
	if detailToken(after, BucketRuntime, "runtime messages") != EstimateTextTokens("summary") {
		t.Fatal("summary is not a runtime message")
	}
}
