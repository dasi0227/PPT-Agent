package llm

// NormalizeHistory prepares historical messages for replay and compaction.
// Old render calls must not teach the model a removed tool argument. This does
// not relax validation of newly generated calls or modify the source messages.
func NormalizeHistory(messages []Message) []Message {
	out := WithoutRenderImages(messages)
	for i, message := range out {
		out[i].ToolCalls = append([]ToolCall(nil), message.ToolCalls...)
		for j, call := range message.ToolCalls {
			if call.Name != "render_slide" {
				continue
			}
			if _, obsolete := call.Args["visual_review"]; !obsolete {
				continue
			}
			args := make(map[string]any, len(call.Args))
			for key, value := range call.Args {
				if key != "visual_review" {
					args[key] = value
				}
			}
			out[i].ToolCalls[j].Args = args
		}
	}
	return out
}
