package service

import (
	"encoding/json"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type renameInput struct {
	CurrentTitle string                       `json:"current_title"`
	Activity     []model.ThreadNamingActivity `json:"activity"`
	Progress     *renameProgress              `json:"progress,omitempty"`
}

type renameProgress struct {
	Title          string           `json:"title"`
	Status         string           `json:"status"`
	CompletedSteps int              `json:"completed_steps"`
	TotalSteps     int              `json:"total_steps"`
	Steps          []renamePlanStep `json:"steps,omitempty"`
}

type renamePlanStep struct {
	Title  string `json:"title"`
	Status string `json:"status"`
}

func buildRenameInput(title string, source model.ThreadRenameContextSource) (string, error) {
	value := renameInput{CurrentTitle: title, Activity: []model.ThreadNamingActivity{}}
	for _, activity := range source.Activity {
		if activity.Role != "user" && activity.Role != "assistant" {
			continue
		}
		if strings.TrimSpace(activity.Text) != "" {
			value.Activity = append(value.Activity, activity)
		}
	}
	if plan := source.Plan; plan != nil {
		progress := &renameProgress{Title: plan.Title, Status: plan.Status, TotalSteps: len(plan.Steps)}
		for _, step := range plan.Steps {
			if step.Status == "completed" {
				progress.CompletedSteps++
			}
			progress.Steps = append(progress.Steps, renamePlanStep{Title: step.Title, Status: step.Status})
		}
		value.Progress = progress
	}
	raw, err := json.Marshal(value)
	return string(raw), err
}
