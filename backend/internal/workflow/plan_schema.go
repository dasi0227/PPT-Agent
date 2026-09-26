package workflow

func planProposalParameters() map[string]any {
	return objectSchema([]string{"title", "content", "steps"}, map[string]any{
		"title":   map[string]any{"type": "string", "minLength": 1},
		"content": map[string]any{"type": "string", "minLength": 1, "description": "Complete plan in Markdown."},
		"steps": map[string]any{"type": "array", "minItems": 1, "items": objectSchema([]string{"title"}, map[string]any{
			"title": map[string]any{"type": "string", "minLength": 1},
			"target_slide_ids": map[string]any{"type": "array", "uniqueItems": true, "items": map[string]any{
				"type": "string", "pattern": "^sli_[A-Za-z0-9_-]+$",
			}},
		})},
	})
}

func planProgressParameters() map[string]any {
	return objectSchema([]string{"updates"}, map[string]any{
		"updates": map[string]any{"type": "array", "minItems": 1, "items": objectSchema([]string{"step_id", "status"}, map[string]any{
			"step_id": map[string]any{"type": "string", "minLength": 1},
			"status":  map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed", "failed"}},
		})},
	})
}
