package llm

import "strings"

func IsRenderImage(part ContentPart) bool {
	return part.Type == "image" && (strings.HasPrefix(part.ImageRef, "run:") ||
		(strings.HasPrefix(part.ImageRef, "project:") && strings.Contains(part.ImageRef, "/render:")))
}

// WithoutRenderImages keeps tool observations and uploaded attachments, but never
// persists or replays runtime screenshot pixels, including existing transcripts.
func WithoutRenderImages(messages []Message) []Message {
	out := make([]Message, len(messages))
	for i, message := range messages {
		out[i] = message
		out[i].Content = make([]ContentPart, 0, len(message.Content))
		for _, part := range message.Content {
			if !IsRenderImage(part) {
				out[i].Content = append(out[i].Content, part)
			}
		}
		if len(out[i].Content) == 0 && len(message.Content) > 0 {
			out[i].Content = TextContent("Rendered image pixels are not retained. Use read_image with the latest runtime image_path when visual inspection is needed.")
		}
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
