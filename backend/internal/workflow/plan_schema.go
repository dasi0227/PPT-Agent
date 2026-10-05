package workflow

func planProposalParameters() map[string]any {
	return objectSchema([]string{"title", "content", "steps"}, map[string]any{
		"title":   map[string]any{"type": "string", "minLength": 1, "description": "User-facing title summarizing the work proposed for approval."},
		"content": map[string]any{"type": "string", "minLength": 1, "description": "Complete Markdown plan for user approval, stating the intended outcome, scope and approach. When revising, provide the full replacement plan, not only the changes."},
		"steps": map[string]any{"type": "array", "minItems": 1, "description": "Ordered execution steps for the complete proposed plan. Runtime assigns step IDs; approval freezes this list.", "items": objectSchema([]string{"title"}, map[string]any{
			"title": map[string]any{"type": "string", "minLength": 1, "description": "A concrete action or deliverable that the user can recognize in plan progress."},
		})},
	})
}

func planProgressParameters() map[string]any {
	return objectSchema([]string{"updates"}, map[string]any{
		"updates": map[string]any{"type": "array", "minItems": 1, "description": "Status changes for existing steps in the approved active plan. Include each step at most once; omitted steps keep their status.", "items": objectSchema([]string{"step_id", "status"}, map[string]any{
			"step_id": map[string]any{"type": "string", "minLength": 1, "description": "Exact step ID supplied in the current plan context, not its title or position."},
			"status":  map[string]any{"type": "string", "enum": []string{"pending", "processing", "completed", "failed"}, "description": "Actual execution state: pending for work not currently underway, processing for current work, completed for finished work, or failed for unsuccessful work. At most one step may be processing; completed steps cannot regress."},
		})},
	})
}
