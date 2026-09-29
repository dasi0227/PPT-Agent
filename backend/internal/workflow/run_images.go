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
	ImageRef    string `json:"image_ref"`
	MIMEType    string `json:"mime_type"`
	Detail      string `json:"detail,omitempty"`
	Observation string `json:"observation,omitempty"`
}

func (state *RunState) rememberReadImages(calls []llm.ToolCall, results []ToolResult) bool {
	seen := make(map[string]bool, len(state.readImages))
	for _, image := range state.readImages {
		seen[image.ImageRef] = true
	}
	added := false
	for i, call := range calls {
		if call.Name != "read_image" || i >= len(results) || !results[i].OK {
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
				ImageRef: part.ImageRef, MIMEType: part.MIMEType, Detail: part.Detail, Observation: observation,
			})
			added = true
		}
	}
	return added
}

// Tool results keep their descriptions and call pairing. The Run collection
// supplies pixels once per reference, even after those results are compacted.
func withoutReadImageParts(messages []llm.Message, images []RunReadImage) []llm.Message {
	if len(images) == 0 {
		return messages
	}
	refs := make(map[string]bool, len(images))
	for _, image := range images {
		refs[image.ImageRef] = true
	}
	out := append([]llm.Message(nil), messages...)
	for i, message := range out {
		if message.Role != llm.RoleTool {
			continue
		}
		parts := make([]llm.ContentPart, 0, len(message.Content))
		for _, part := range message.Content {
			if part.Type != "image" || !refs[part.ImageRef] {
				parts = append(parts, part)
			}
		}
		if len(parts) == 0 && len(message.Content) > 0 {
			parts = llm.TextContent("The read image is retained in this Run's image context.")
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
	label := map[string]any{"image_ref": image.ImageRef, "observation_at_read": image.Observation}
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
	var original RenderedImageContext
	if json.Unmarshal([]byte(image.Observation), &original) != nil || original.SlideID == "" {
		return "unknown"
	}
	for _, current := range latest {
		if current.SlideID != original.SlideID {
			continue
		}
		if current.ImagePath != original.ImagePath {
			return "superseded"
		}
		if current.Stale {
			return "stale"
		}
		return "current"
	}
	return "not_in_current_index"
}
