package workflow

import (
	"encoding/json"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

// RunReadImage owns an immutable image reference for one Run. Bytes remain in
// the project store and are resolved by the provider adapter on each request.
// Observation is the tool's original description, not a visual judgement.
type RunReadImage struct {
	CallID       string `json:"call_id"`
	SlideID      string `json:"slide_id,omitempty"`
	AttachmentID string `json:"attachment_id,omitempty"`
	ImagePath    string `json:"image_path,omitempty"`
	ImageRef     string `json:"image_ref"`
	MIMEType     string `json:"mime_type"`
	Detail       string `json:"detail,omitempty"`
	Observation  string `json:"observation,omitempty"`
}

func (state *RunState) rememberReadImages(calls []llm.ToolCall, results []ToolResult) bool {
	seen := make(map[string]bool, len(state.readImages))
	for _, image := range state.readImages {
		seen[image.ImageRef] = true
	}
	added := false
	for i, call := range calls {
		if (call.Name != "read_image" && call.Name != "render_slide") || i >= len(results) {
			continue
		}
		result := results[i]
		observation := result.Observation
		if observation == "" {
			observation = (llm.Message{Content: result.ObservationParts}).Text()
		}
		for _, part := range result.ObservationParts {
			if part.Type != "image" || part.ImageRef == "" || seen[part.ImageRef] {
				continue
			}
			seen[part.ImageRef] = true
			state.readImages = append(state.readImages, RunReadImage{
				CallID: call.ID, SlideID: stringValue(call.Args["slide_id"]), AttachmentID: stringValue(call.Args["attachment_id"]), ImagePath: stringValue(result.Data["image_path"]), ImageRef: part.ImageRef, MIMEType: part.MIMEType, Detail: part.Detail, Observation: observation,
			})
			added = true
		}
	}
	return added
}

// Keep pixels in the original tool response. Only older copies of a repeated
// read are compacted; the newest call always returns its actual image block.
// appendRunImages restores retained references only after compaction removes them.
func withoutReadImageParts(messages []llm.Message, images []RunReadImage) []llm.Message {
	if len(images) == 0 {
		return messages
	}
	refs := map[string]bool{}
	for _, image := range images {
		refs[image.ImageRef] = true
	}
	seen := map[string]bool{}
	out := append([]llm.Message(nil), messages...)
	for i := len(out) - 1; i >= 0; i-- {
		message := out[i]
		if message.Role != llm.RoleTool {
			continue
		}
		parts := make([]llm.ContentPart, 0, len(message.Content))
		for _, part := range message.Content {
			if part.Type == "image" && refs[part.ImageRef] {
				if seen[part.ImageRef] {
					continue
				}
				seen[part.ImageRef] = true
			}
			parts = append(parts, part)
		}
		if len(parts) == 0 && len(message.Content) > 0 {
			parts = llm.TextContent("Image retained in the later tool response.")
		}
		out[i].Content = parts
	}
	return out
}

func appendRunImages(req AgentRequest) []llm.Message {
	byRef := make(map[string]RunReadImage, len(req.ReadImages))
	for _, image := range req.ReadImages {
		byRef[image.ImageRef] = image
	}
	messages := make([]llm.Message, 0, len(req.Messages)+len(req.ReadImages))
	seen := map[string]bool{}
	// A user-selected attachment may already be present in ordinary history.
	for _, message := range req.Messages {
		if m := message.Metadata; m != nil && m.Origin == "runtime" && m.Kind == "run_image" {
			continue
		}
		for _, part := range message.Content {
			if part.Type == "image" {
				seen[part.ImageRef] = true
			}
		}
	}
	// Keep existing image messages in place. Moving them after each new tool
	// round would rewrite the prompt prefix and invalidate provider replay.
	for _, message := range req.Messages {
		m := message.Metadata
		if m == nil || m.Origin != "runtime" || m.Kind != "run_image" {
			messages = append(messages, message)
			continue
		}
		image, exists := byRef[m.Key]
		if exists && m.RunID == req.RunID && !seen[image.ImageRef] {
			messages = append(messages, runImageMessage(req, image))
			seen[image.ImageRef] = true
		}
	}
	for _, image := range req.ReadImages {
		if seen[image.ImageRef] {
			continue
		}
		seen[image.ImageRef] = true
		messages = append(messages, runImageMessage(req, image))
	}
	return messages
}

func runImageMessage(req AgentRequest, image RunReadImage) llm.Message {
	label := map[string]any{}
	if image.SlideID != "" {
		label["slide_id"] = image.SlideID
	}
	if image.AttachmentID != "" {
		label["attachment_id"] = image.AttachmentID
	}
	if strings.Contains(image.ImageRef, "/render:") {
		label["render_state"] = retainedRenderState(image, req.RenderedImages)
	}
	raw, _ := json.Marshal(label)
	return llm.Message{
		Role: llm.RoleUser,
		Content: []llm.ContentPart{
			{Type: "text", Text: "<run_read_image>" + string(raw) + "</run_read_image>"},
			{Type: "image", ImageRef: image.ImageRef, MIMEType: image.MIMEType, Detail: image.Detail},
		},
		Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "run_image", RunID: req.RunID, Key: image.ImageRef},
	}
}

func retainedRenderState(image RunReadImage, latest []RenderedImageContext) string {
	if image.SlideID == "" {
		return "unknown"
	}
	for _, current := range latest {
		if current.SlideID != image.SlideID {
			continue
		}
		if current.ImagePath != image.ImagePath {
			return "superseded"
		}
		if current.Stale {
			return "stale"
		}
		return "current"
	}
	return "not_in_current_index"
}
