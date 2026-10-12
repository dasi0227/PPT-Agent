package service

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
)

type renameInput struct {
	CurrentTitle   string                       `json:"current_title"`
	RecentActivity []model.ThreadNamingActivity `json:"recent_activity"`
	Progress       *renameProgress              `json:"progress,omitempty"`
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
	value := renameInput{CurrentTitle: commandExcerpt(title, 60), RecentActivity: []model.ThreadNamingActivity{}}
	latestUser := -1
	for i, activity := range source.Activity {
		if activity.Role == "user" && strings.TrimSpace(activity.Text) != "" {
			latestUser = i
		}
	}
	users, replies, start := 0, 0, len(source.Activity)
	for start > 0 && users < 4 {
		start--
		if source.Activity[start].Role == "user" && strings.TrimSpace(source.Activity[start].Text) != "" {
			users++
		}
	}
	// Keep the newest two final replies while retaining chronological order.
	for i := len(source.Activity) - 1; i >= start; i-- {
		activity := source.Activity[i]
		limit := 400
		switch activity.Role {
		case "user":
			if i == latestUser {
				limit = 800
			}
		case "assistant":
			if replies >= 2 {
				continue
			}
			replies++
			limit = 360
		default:
			continue
		}
		activity.Text = commandExcerpt(activity.Text, limit)
		if activity.Text != "" {
			value.RecentActivity = append([]model.ThreadNamingActivity{activity}, value.RecentActivity...)
		}
	}
	if plan := source.Plan; plan != nil {
		progress := &renameProgress{Title: commandExcerpt(plan.Title, 120), Status: commandExcerpt(plan.Status, 32), TotalSteps: len(plan.Steps)}
		for _, step := range plan.Steps {
			if step.Status == "completed" {
				progress.CompletedSteps++
			}
		}
		// Active work first, then the next pending step, then recent completions.
		// Never include plan content, IDs, page targets or an unbounded checklist.
		for _, status := range []string{"processing", "failed", "pending", "completed"} {
			for offset := range plan.Steps {
				index := offset
				if status == "completed" {
					index = len(plan.Steps) - 1 - offset
				}
				step := plan.Steps[index]
				if step.Status == status && len(progress.Steps) < 3 {
					progress.Steps = append(progress.Steps, renamePlanStep{Title: commandExcerpt(step.Title, 100), Status: status})
					if status == "pending" {
						break
					}
				}
			}
		}
		value.Progress = progress
	}
	return fitRenameInput(value)
}

func fitRenameInput(value renameInput) (string, error) {
	// Account for the actual system prompt and tool, not a separate estimate of
	// a larger agent policy. Trim oldest activity first; preserve the latest intent.
	system := prompts.MustLoad("command.rename").Body
	for {
		raw, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		request := llm.GenerateRequest{Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: llm.TextContent(system)},
			{Role: llm.RoleUser, Content: llm.TextContent(string(raw))},
		}, Tools: []llm.ToolSchema{renameThreadToolSchema()}}
		if llm.EstimateRequestTokens(request) <= renameInputBudget {
			return string(raw), nil
		}
		if len(value.RecentActivity) > 2 {
			latestUser := -1
			for i, activity := range value.RecentActivity {
				if activity.Role == "user" {
					latestUser = i
				}
			}
			drop := 0
			if drop == latestUser {
				drop++
			}
			value.RecentActivity = append(value.RecentActivity[:drop], value.RecentActivity[drop+1:]...)
			continue
		}
		if value.Progress != nil {
			value.Progress = nil
			continue
		}
		trimmed := false
		for i, activity := range value.RecentActivity {
			if text := commandExcerpt(activity.Text, 240); text != activity.Text {
				value.RecentActivity[i].Text = text
				trimmed = true
			}
		}
		if !trimmed {
			return "", errors.New("rename prompt exceeds input budget")
		}
	}
}
