package llm

import "strings"

func IsRenderImage(part ContentPart) bool {
	return part.Type == "image" && (strings.HasPrefix(part.ImageRef, "run:") ||
		(strings.HasPrefix(part.ImageRef, "project:") && strings.Contains(part.ImageRef, "/render:")))
}

// WithoutRenderImages removes screenshot parts from ordinary thread history.
// Run-owned image references are restored separately from the Run checkpoint.
func WithoutRenderImages(messages []Message) []Message {
	out := make([]Message, len(messages))
	for i, message := range messages {
		out[i] = message
		out[i].Content = make([]ContentPart, 0, len(message.Content))
		for _, part := range message.Content {
			if !IsRenderImage(part) && !(message.Role == RoleTool && part.Type == "image") {
				out[i].Content = append(out[i].Content, part)
			}
		}
		if len(out[i].Content) == 0 && len(message.Content) > 0 {
			out[i].Content = TextContent("Historical image pixels are omitted. Use read_image with slide_id or attachment_id; render the page first if its screenshot is missing or stale.")
		}
	}
	return out
}

// WithoutRunImageMessages removes derived image context from durable thread
// history and compaction input. It is reconstructed from the active Run only.
func WithoutRunImageMessages(messages []Message) []Message {
	out := make([]Message, 0, len(messages))
	for _, message := range messages {
		if m := message.Metadata; m != nil && m.Origin == "runtime" && m.Kind == "run_image" {
			continue
		}
		out = append(out, message)
	}
	return out
}

func HasRenderImages(messages []Message) bool {
	for _, message := range messages {
		for _, part := range message.Content {
			if IsRenderImage(part) {
				return true
			}
		}
	}
	return false
}
