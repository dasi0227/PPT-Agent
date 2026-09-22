package workflow

import (
	"context"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type ephemeralImageProvider struct{}

func (ephemeralImageProvider) RegisterDomainTools(registry *ToolRegistry) error {
	return registry.RegisterDomainTool(ephemeralImageTool{}, true, PhaseChat)
}

type ephemeralImageTool struct{}

func (ephemeralImageTool) Schema() ToolSchema {
	return ToolSchema{Name: "read_image", Description: "read rendered image", Parameters: objectSchema(nil, map[string]any{})}
}

func (ephemeralImageTool) Execute(context.Context, DomainToolInput) ToolResult {
	result := SuccessfulToolResult("render read")
	result.ObservationParts = []llm.ContentPart{
		{Type: "text", Text: "latest rendered image"},
		{Type: "image", ImageRef: "project:p1/render:sli_1/shot_one", MIMEType: "image/png"},
	}
	return result
}

type imageLifecycleTranscript struct {
	recordingTranscript
	persistedPixels bool
}

func (s *imageLifecycleTranscript) Replace(project, thread string, messages []llm.Message) error {
	s.persistedPixels = s.persistedPixels || llm.HasRenderImages(messages)
	return s.recordingTranscript.Replace(project, thread, messages)
}

func TestRenderReadIsVisibleForOneResponseAndNeverPersisted(t *testing.T) {
	transcript := &imageLifecycleTranscript{recordingTranscript: recordingTranscript{messages: []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentPart{
			{Type: "text", Text: "previous render and uploaded reference"},
			{Type: "image", ImageRef: "run:old/screenshot:shot_old"},
			{Type: "image", ImageRef: "project:p1/attachment:att_one/original"},
		}},
	}}}
	agent := &scriptedAgent{responses: []AgentResponse{
		toolCall("read", "read_image", map[string]any{}),
		{Text: "visual finding: heading needs more spacing", Continuation: &llm.ProviderContinuation{Provider: "test", Model: "test"}},
		finishCall("finish"),
	}}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "image-lifecycle", ProjectDir: t.TempDir(), Transcript: transcript,
		Context:     testPack(model.ModeChat, model.ScopeObjectSpec, model.ScopeCurrentPage, false, "inspect the slide"),
		DomainTools: ephemeralImageProvider{},
	})
	if outcome.Status != StatusCompleted || len(agent.requests) != 3 {
		t.Fatalf("unexpected lifecycle: %+v, requests=%d", outcome, len(agent.requests))
	}
	for index, request := range agent.requests {
		if llm.HasRenderImages(request.Messages) != (index == 1) {
			t.Fatalf("render pixels in wrong request %d: %+v", index, request.Messages)
		}
		attachmentKept := false
		for _, message := range request.Messages {
			for _, part := range message.Content {
				attachmentKept = attachmentKept || part.ImageRef == "project:p1/attachment:att_one/original"
			}
		}
		if !attachmentKept {
			t.Fatalf("upload removed in request %d", index)
		}
	}
	if agent.requests[2].Continuation != nil || transcript.persistedPixels {
		t.Fatal("render pixels retained through continuation or transcript")
	}
	findingKept := false
	for _, message := range transcript.messages {
		findingKept = findingKept || strings.Contains(message.Text(), "visual finding:")
	}
	if !findingKept {
		t.Fatal("visual finding was not persisted as text")
	}
}
