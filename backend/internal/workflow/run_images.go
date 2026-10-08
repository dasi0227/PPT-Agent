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
			image := RunReadImage{
				CallID: call.ID, SlideID: stringValue(call.Args["slide_id"]), AttachmentID: stringValue(call.Args["attachment_id"]), ImagePath: stringValue(result.Data["image_path"]), ImageRef: part.ImageRef, MIMEType: part.MIMEType, Detail: part.Detail, Observation: observation,
			}
			if llm.IsRenderImage(part) {
				retained := state.readImages[:0]
				for _, previous := range state.readImages {
					if previous.SlideID != image.SlideID || !isRunRender(previous) {
						retained = append(retained, previous)
					}
				}
				state.readImages = retained
			}
			state.readImages = append(state.readImages, image)
			added = true
		}
	}
	return added
}

func isRunRender(image RunReadImage) bool {
	return llm.IsRenderImage(llm.ContentPart{Type: "image", ImageRef: image.ImageRef})
}

// Each page contributes only its current valid screenshot. Uploaded originals
// remain independent of page renders and are retained as before.
func currentRunImages(images []RunReadImage, latest []RenderedImageContext) []RunReadImage {
	current := make(map[string]string, len(latest))
	for _, image := range latest {
		if !image.Stale {
			current[image.SlideID] = image.ImagePath
		}
	}
	seenRefs, seenSlides := map[string]bool{}, map[string]bool{}
	keep := make([]bool, len(images))
	for i := len(images) - 1; i >= 0; i-- {
		image := images[i]
		if seenRefs[image.ImageRef] {
			continue
		}
		if isRunRender(image) {
			path, exists := current[image.SlideID]
			if !exists || path != image.ImagePath || seenSlides[image.SlideID] {
				continue
			}
			seenSlides[image.SlideID] = true
		}
		keep[i], seenRefs[image.ImageRef] = true, true
	}
	out := make([]RunReadImage, 0, len(images))
	for i, image := range images {
		if keep[i] {
			out = append(out, image)
		}
	}
	return out
}

// Keep each retained image in its newest tool response and remove obsolete
// screenshot pixels from every older message without changing tool pairing.
func withoutReadImageParts(messages []llm.Message, images []RunReadImage) []llm.Message {
	refs := map[string]bool{}
	for _, image := range images {
		refs[image.ImageRef] = true
	}
	seen := map[string]bool{}
	out := append([]llm.Message(nil), messages...)
	for i := len(out) - 1; i >= 0; i-- {
		message := out[i]
		parts := make([]llm.ContentPart, 0, len(message.Content))
		for _, part := range message.Content {
			if llm.IsRenderImage(part) && !refs[part.ImageRef] {
				continue
			}
			if part.Type == "image" && refs[part.ImageRef] && message.Role == llm.RoleTool {
				if seen[part.ImageRef] {
					continue
				}
				seen[part.ImageRef] = true
			}
			parts = append(parts, part)
		}
		if len(parts) == 0 && len(message.Content) > 0 {
			parts = llm.TextContent("Image pixels are omitted from this earlier message.")
		}
		out[i].Content = parts
	}
	return out
}

func appendRunImages(req AgentRequest) []llm.Message {
	req.Messages = withoutReadImageParts(req.Messages, req.ReadImages)
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
		label["render_state"] = "current"
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
