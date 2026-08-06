package workflow

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

const (
	RequirementPending    = "pending"
	RequirementInProgress = "in_progress"
	RequirementSatisfied  = "satisfied"
)

type RequirementItem struct {
	ID       string   `json:"id"`
	Text     string   `json:"text"`
	Status   string   `json:"status"`
	Evidence []string `json:"evidence,omitempty"`
}

type RequirementLedger struct {
	Items []RequirementItem `json:"items"`
}

func NewRequirementLedger(spec model.WorkSpec) *RequirementLedger {
	parts := splitRequirements(spec.Instruction)
	if len(parts) == 0 {
		parts = []string{strings.TrimSpace(spec.Instruction)}
	}
	items := make([]RequirementItem, 0, len(parts)+1)
	for index, part := range parts {
		items = append(items, RequirementItem{
			ID: fmt.Sprintf("req_%02d", index+1), Text: part, Status: RequirementPending,
		})
	}
	target := fmt.Sprintf("Target artifact=%s level=%s", spec.Target.Artifact, spec.Target.Level)
	if spec.Target.SlideID != "" {
		target += " slide_id=" + spec.Target.SlideID
	}
	items = append(items, RequirementItem{ID: "req_target_scope", Text: target, Status: RequirementSatisfied})
	return &RequirementLedger{Items: items}
}

func splitRequirements(instruction string) []string {
	clean := strings.TrimSpace(instruction)
	if clean == "" {
		return nil
	}
	fields := strings.FieldsFunc(clean, func(r rune) bool {
		switch r {
		case '\n', '\r', ';', '；', '。', '，', ',', '、':
			return true
		default:
			return unicode.IsControl(r)
		}
	})
	out := []string{}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len([]rune(field)) < 2 {
			continue
		}
		out = append(out, field)
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func (l *RequirementLedger) ObserveToolResults(results []ToolResult) {
	if l == nil {
		return
	}
	for _, result := range results {
		if !result.OK {
			continue
		}
		for _, target := range result.ChangedTargets {
			l.markAll(RequirementSatisfied, "changed:"+target.Target().Key())
		}
		for _, evidence := range result.Evidence {
			if evidence.Fresh || evidence.Kind != "" {
				l.markAll(RequirementSatisfied, "evidence:"+evidence.Kind+":"+evidence.Target.Key())
			}
		}
	}
}

func (l *RequirementLedger) MarkInProgress(note string) {
	if l == nil {
		return
	}
	l.markAll(RequirementInProgress, note)
}

func (l *RequirementLedger) markAll(status string, evidence string) {
	for index := range l.Items {
		if l.Items[index].Status == RequirementSatisfied {
			continue
		}
		l.Items[index].Status = status
		if evidence != "" && !containsString(l.Items[index].Evidence, evidence) {
			l.Items[index].Evidence = append(l.Items[index].Evidence, evidence)
		}
	}
}

func (l *RequirementLedger) BlockingItems() []RequirementItem {
	if l == nil {
		return nil
	}
	out := []RequirementItem{}
	for _, item := range l.Items {
		if item.Status == RequirementPending {
			out = append(out, item)
		}
	}
	return out
}

func (l *RequirementLedger) Brief() string {
	if l == nil || len(l.Items) == 0 {
		return "- none"
	}
	lines := make([]string, 0, len(l.Items))
	for _, item := range l.Items {
		line := fmt.Sprintf("- %s [%s]: %s", item.ID, item.Status, item.Text)
		if len(item.Evidence) > 0 {
			line += " evidence=" + strings.Join(item.Evidence, ",")
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func (p Plan) Brief() string {
	if len(p.Steps) == 0 {
		return "empty"
	}
	counts := map[PlanStepStatus]int{}
	parts := make([]string, 0, len(p.Steps))
	for _, step := range p.Steps {
		counts[step.Status]++
		if len(parts) < 4 {
			parts = append(parts, fmt.Sprintf("%s:%s", step.ID, step.Status))
		}
	}
	return fmt.Sprintf("revision=%d completed=%d pending=%d in_progress=%d failed=%d steps=%s",
		p.Revision, counts[PlanStepCompleted], counts[PlanStepPending], counts[PlanStepInProgress], counts[PlanStepFailed], strings.Join(parts, ", "))
}
