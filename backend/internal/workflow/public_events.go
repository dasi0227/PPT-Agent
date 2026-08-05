package workflow

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

var (
	htmlTagPattern   = regexp.MustCompile(`(?is)<(?:html|body|section|script|style|iframe|svg)\b`)
	rawHTMLPattern   = regexp.MustCompile(`(?i)<\s*/?\s*[a-z][a-z0-9-]*(?:\s+[^>]*)?/?\s*>`)
	localPathPattern = regexp.MustCompile(`(?:^|\s)(?:/Users/|/home/|/tmp/|[A-Za-z]:\\)`)
)

func publicBase(runID string) model.PublicEventBase {
	return model.NewPublicEventBase(runID)
}

func publicTarget(target Resource) model.PublicTarget {
	return model.PublicTarget{Type: target.Type, SlideID: target.SlideID, Part: target.Part}
}

func publicAffectedTargets(changes ChangeSet) []model.PublicTarget {
	seen := map[string]model.PublicTarget{}
	for _, change := range changes.All() {
		target := publicTarget(resourceForArtifact(change.Artifact))
		seen[target.Type+":"+target.SlideID+":"+target.Part] = target
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]model.PublicTarget, 0, len(keys))
	for _, key := range keys {
		out = append(out, seen[key])
	}
	return out
}

func publicPlan(plan Plan) model.PublicPlan {
	steps := make([]model.PublicPlanStep, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		steps = append(steps, model.PublicPlanStep{
			ID: step.ID, Title: sanitizePublicText(step.Title, 120), Status: string(step.Status),
		})
	}
	return model.PublicPlan{
		PlanID: plan.ID, Revision: plan.Revision,
		Explanation: sanitizePublicText(plan.Explanation, 180), Steps: steps,
	}
}

func completedPlanSteps(previous *Plan, next Plan) []PlanStep {
	if previous == nil {
		return nil
	}
	wasCompleted := map[string]bool{}
	for _, step := range previous.Steps {
		wasCompleted[step.ID] = step.Status == PlanStepCompleted
	}
	out := []PlanStep{}
	for _, step := range next.Steps {
		if step.Status == PlanStepCompleted && !wasCompleted[step.ID] {
			out = append(out, step)
		}
	}
	return out
}

func milestoneText(explanation string, completed []PlanStep) string {
	if text := sanitizePublicText(explanation, 180); text != "" {
		return text
	}
	titles := make([]string, 0, len(completed))
	for _, step := range completed {
		titles = append(titles, strings.TrimSpace(step.Title))
	}
	if len(titles) == 1 {
		return titles[0] + "已经完成。"
	}
	return strings.Join(titles, "、") + "已经完成。"
}

func sanitizePublicReasoning(text string) string {
	text = sanitizePublicText(text, 160)
	lower := strings.ToLower(text)
	if text == "" ||
		htmlTagPattern.MatchString(text) ||
		localPathPattern.MatchString(text) ||
		strings.Contains(lower, "reasoning_content") ||
		strings.Contains(lower, "chain of thought") ||
		strings.Contains(text, "思维链") ||
		strings.Contains(text, "系统提示词") ||
		strings.Contains(text, "密钥") {
		return ""
	}
	return text
}

func sanitizePublicText(text string, maxRunes int) string {
	text = rawHTMLPattern.ReplaceAllString(text, "")
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) > maxRunes {
		text = string(runes[:maxRunes])
		if utf8.RuneCountInString(text) > 0 {
			text += "…"
		}
	}
	return text
}

type ToolPublicProjector struct{}

func (ToolPublicProjector) Started(runID, callID, tool string, args map[string]any, planStepID string) (model.ToolStartedPayload, bool) {
	target := publicToolTarget(tool, args)
	label, detail, ok := toolDisplay(tool, args, true, ToolResult{})
	if !ok {
		return model.ToolStartedPayload{}, false
	}
	return model.ToolStartedPayload{
		PublicEventBase: publicBase(runID),
		CallID:          callID, Tool: tool, PlanStepID: planStepID,
		Target: target, Display: model.PublicDisplay{Label: label, Detail: detail},
	}, true
}

func (ToolPublicProjector) Completed(runID, callID, tool string, args map[string]any, result ToolResult) (model.ToolCompletedPayload, bool) {
	label, detail, ok := toolDisplay(tool, args, false, result)
	if !ok {
		return model.ToolCompletedPayload{}, false
	}
	status := "completed"
	if !result.OK {
		status = "failed"
	}
	payload := model.ToolCompletedPayload{
		PublicEventBase: publicBase(runID),
		CallID:          callID, Tool: tool, Status: status,
		Target:  publicToolTarget(tool, args),
		Display: model.PublicDisplay{Label: label, Detail: detail},
	}
	if !result.OK {
		agentErr := model.NewAgentError(publicErrorCode(result.Code), "tool_call", nil)
		payload.Error = agentErr.Public()
	}
	if tool == "render_slide" {
		payload.Preview = publicRenderPreview(runID, args, result)
	}
	return payload, true
}

func publicToolTarget(tool string, args map[string]any) *model.PublicTarget {
	if tool == "render_slide" {
		slideID := stringValue(args["slide_id"])
		if slideID != "" {
			return &model.PublicTarget{Type: "slide", SlideID: slideID, Part: "html"}
		}
	}
	target, _ := args["resource"].(map[string]any)
	targetType := stringValue(target["type"])
	if targetType == "deck" {
		return &model.PublicTarget{Type: "deck", Part: stringValue(target["part"])}
	}
	if targetType == "slide" {
		return &model.PublicTarget{
			Type: "slide", SlideID: stringValue(target["slide_id"]), Part: stringValue(target["part"]),
		}
	}
	return nil
}

func toolDisplay(tool string, args map[string]any, started bool, result ToolResult) (string, string, bool) {
	target := publicToolTarget(tool, args)
	targetName := "内容"
	if target != nil && target.Type == "deck" && target.Part == "outline" {
		targetName = "整份结构"
	} else if target != nil && target.Type == "deck" && target.Part == "design" {
		targetName = "全局设计"
	} else if target != nil && target.Type == "slide" {
		if target.Part == "spec" {
			targetName = slideDisplayName(target.SlideID) + "设计稿"
		} else {
			targetName = slideDisplayName(target.SlideID) + "幻灯片"
		}
	}
	switch tool {
	case "read_ppt":
		if started {
			return "读取" + targetName, "确认内容与设计约束", true
		}
		if result.OK {
			return "已读取" + targetName, safeToolDetail(result, "已获得所需内容"), true
		}
		return "读取" + targetName + "失败", publicToolError(result), true
	case "write_ppt":
		if started {
			return "创建" + targetName, "", true
		}
		if result.OK {
			return "已创建" + targetName, safeToolDetail(result, "内容已写入安全暂存区"), true
		}
		return "创建" + targetName + "失败", publicToolError(result), true
	case "edit_ppt":
		if started {
			return "更新" + targetName, "", true
		}
		if result.OK {
			return "已更新" + targetName, safeToolDetail(result, "修改已写入安全暂存区"), true
		}
		return "更新" + targetName + "失败", publicToolError(result), true
	case "search_refs":
		query := sanitizePublicText(stringValue(args["query"]), 48)
		if query == "" {
			query = "相关设计参考"
		}
		if started {
			return "查找" + query, "", true
		}
		return "已完成参考检索", safeToolDetail(result, "已获得相关参考"), true
	case "render_slide":
		if started {
			return "检查" + targetName + "布局", "", true
		}
		if result.OK {
			return targetName + "渲染通过", renderDetail(result), true
		}
		return targetName + "渲染未通过", publicToolError(result), true
	default:
		return "", "", false
	}
}

func safeToolDetail(result ToolResult, fallback string) string {
	if !result.OK {
		return publicToolError(result)
	}
	text := sanitizePublicText(result.Summary, 100)
	if text == "" || localPathPattern.MatchString(text) || htmlTagPattern.MatchString(text) {
		return fallback
	}
	return text
}

func publicToolError(result ToolResult) string {
	agentErr := model.NewAgentError(publicErrorCode(result.Code), "tool_call", nil)
	if text := sanitizePublicText(agentErr.SafeMessage, 120); text != "" {
		return text
	}
	return "工具未能完成，请调整后重试。"
}

func publicErrorCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return "TOOL_FAILED"
	}
	return code
}

func renderDetail(result ToolResult) string {
	warnings := publicWarnings(result)
	if len(warnings) == 0 {
		return "未发现溢出或裁切"
	}
	return fmt.Sprintf("发现 %d 项布局提示", len(warnings))
}

func publicRenderPreview(runID string, args map[string]any, result ToolResult) *model.ToolPreview {
	slideID := stringValue(args["slide_id"])
	if slideID == "" {
		return nil
	}
	imageURL := ""
	if result.Data != nil {
		imageURL = stringValue(result.Data["screenshot_url"])
	}
	if !strings.HasPrefix(imageURL, "/api/v1/runs/"+runID+"/screenshots/") {
		imageURL = ""
	}
	if imageURL == "" {
		return nil
	}
	return &model.ToolPreview{
		SlideID: slideID, ImageURL: imageURL, Warnings: publicWarnings(result),
	}
}

func publicWarnings(result ToolResult) []string {
	out := []string{}
	for _, issue := range result.Issues {
		if issue.Severity == SeverityWarning {
			if text := sanitizePublicText(issue.Summary, 100); text != "" {
				out = append(out, text)
			}
		}
	}
	return out
}

func slideDisplayName(slideID string) string {
	slideID = sanitizePublicText(slideID, 40)
	if slideID == "" {
		return "页面"
	}
	return "页面 " + slideID
}

func newMessageID() string {
	return "msg_" + uuid.NewString()
}

func publicQuestion(runID, questionID string, args map[string]any) model.QuestionAskedPayload {
	options := []model.QuestionOption{}
	rawOptions, _ := args["options"].([]any)
	for index, raw := range rawOptions {
		option, _ := raw.(map[string]any)
		label := sanitizePublicText(stringValue(option["label"]), 80)
		if label == "" {
			continue
		}
		id := sanitizePublicText(stringValue(option["id"]), 64)
		if id == "" {
			id = fmt.Sprintf("option-%d", index+1)
		}
		options = append(options, model.QuestionOption{
			ID: id, Label: label,
			Description: sanitizePublicText(stringValue(option["description"]), 140),
		})
	}
	selection := "single"
	if multiple, _ := args["multiple"].(bool); multiple {
		selection = "multiple"
	}
	allowCustom, _ := args["allow_custom"].(bool)
	if len(options) == 0 {
		allowCustom = true
	}
	return model.QuestionAskedPayload{
		PublicEventBase: publicBase(runID), QuestionID: questionID,
		Header:    sanitizePublicText(stringValue(args["header"]), 24),
		Prompt:    sanitizePublicText(stringValue(args["question"]), 240),
		Selection: selection, Options: options, AllowCustom: allowCustom,
	}
}

func currentPlanStepID(plan *Plan) string {
	if plan == nil {
		return ""
	}
	for _, step := range plan.Steps {
		if step.Status == PlanStepInProgress {
			return step.ID
		}
	}
	return ""
}

func safeFinalMessage(message string, strategy ExecutionStrategy, affected int) string {
	if text := sanitizePublicText(message, 1200); text != "" &&
		!htmlTagPattern.MatchString(text) && !localPathPattern.MatchString(text) {
		return text
	}
	if strategy == StrategyTalk || strategy == StrategyAsk {
		return "已完成本次分析。"
	}
	if strategy == StrategyPlan {
		return "已完成本次计划。"
	}
	if affected > 0 {
		return fmt.Sprintf("已完成本次修改并检查了 %d 个受影响目标。", affected)
	}
	return "已完成本次任务。"
}

func reasoningDuplicate(previous, next string) bool {
	left := strings.ToLower(strings.TrimSpace(previous))
	right := strings.ToLower(strings.TrimSpace(next))
	if left == "" || right == "" {
		return false
	}
	if left == right {
		return true
	}
	shorter, longer := left, right
	if len(shorter) > len(longer) {
		shorter, longer = longer, shorter
	}
	return len(shorter) >= 24 && strings.Contains(longer, shorter)
}
