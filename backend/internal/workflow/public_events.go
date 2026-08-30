package workflow

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func publicBase(runID string) model.PublicEventBase {
	return model.NewPublicEventBase(runID)
}

func publicTarget(projectDir string, target Resource) model.PublicTarget {
	out := model.PublicTarget{Type: target.Type, SlideID: target.SlideID, Part: target.Part}
	if target.Type == "slide" {
		out.DisplayName = runtimeSlideDisplayName(projectDir, target.SlideID)
	}
	if target.Type == "file" {
		out.SlideID = ""
		out.DisplayName = target.Path
		return out
	}
	attachLocalOpenTarget(projectDir, &out)
	return out
}

func publicAffectedTargets(projectDir string, changes ChangeSet) []model.PublicTarget {
	seen := map[string]model.PublicTarget{}
	for _, change := range changes.All() {
		target := publicTarget(projectDir, resourceForArtifact(change.Artifact))
		target.Insertions = change.Insertions
		target.Deletions = change.Deletions
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

func publicChangedTargets(projectDir string, changes []ChangedTarget) []model.PublicTarget {
	seen := map[string]model.PublicTarget{}
	for _, change := range changes {
		target := model.PublicTarget{
			Type: change.Type, SlideID: change.SlideID, Part: change.Part,
			Insertions: change.Insertions, Deletions: change.Deletions,
		}
		if target.Type == "file" {
			target.DisplayName = change.DisplayName
		}
		if target.Type == "slide" {
			target.DisplayName = runtimeSlideDisplayName(projectDir, target.SlideID)
		}
		attachLocalOpenTarget(projectDir, &target)
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
		PlanID: plan.ID, Revision: plan.Revision, ApprovedRevision: plan.ApprovedRevision,
		Status: string(plan.Status), Title: sanitizePublicText(plan.Title, 180), Content: sanitizePublicMarkdown(plan.Content, 12000), Steps: steps,
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

func planStepIDs(steps []PlanStep) []string {
	ids := make([]string, 0, len(steps))
	for _, step := range steps {
		ids = append(ids, step.ID)
	}
	return ids
}

func milestoneText(title string, completed []PlanStep) string {
	titles := make([]string, 0, len(completed))
	for _, step := range completed {
		if t := strings.TrimSpace(step.Title); t != "" {
			titles = append(titles, t)
		}
	}
	if len(titles) > 0 {
		return strings.Join(titles, "、") + "已完成"
	}
	return sanitizePublicText(title, 180)
}

func sanitizePublicReasoning(text string) string {
	// Reasoning tolerates engineering vocabulary more than final delivery does;
	// the frontend already frames it as a collapsed thinking trace. Keep it lenient
	// and only normalize whitespace, so we never distort the model's own wording.
	return normalizePublicText(text)
}

func sanitizePublicText(text string, _ int) string {
	return redactInternalTerms(normalizePublicText(text))
}

func sanitizePublicMarkdown(text string, _ int) string {
	return redactInternalTerms(normalizePublicText(text))
}

func normalizePublicText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.TrimSpace(text)
}

// internalTermReplacements is the Layer 3 fallback: when a leak slips past the
// user-facing output law (Layer 2), we still translate the highest-signal
// engineering tokens into product language before they reach the user. Patterns
// are ordered specific-before-generic and only match shapes that cannot collide
// with ordinary Chinese prose (resource keys, snake_case identifiers, ALL_CAPS
// error codes, RunX jargon).
var internalTermReplacements = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	// Resource display keys — match the compound slide forms before deck forms.
	{regexp.MustCompile(`(?i)\bslide:[A-Za-z0-9_-]+:spec\b`), "页面设计稿"},
	{regexp.MustCompile(`(?i)\bslide:[A-Za-z0-9_-]+:html\b`), "幻灯片页面"},
	{regexp.MustCompile(`(?i)\bdeck:manifest\b`), "演示内容"},
	{regexp.MustCompile(`(?i)\bdeck:outline\b`), "目录结构"},
	{regexp.MustCompile(`(?i)\bdeck:design\b`), "视觉设计"},
	// Tool and control action identifiers.
	{regexp.MustCompile(`\b(?:read_ppt|mutate_ppt)\b`), "PPT 内容操作"},
	{regexp.MustCompile(`\brender_slide\b`), "页面渲染检查"},
	{regexp.MustCompile(`\b(?:create_plan|update_plan)\b`), "计划"},
	{regexp.MustCompile(`\breview_completion\b`), "完成检查"},
	{regexp.MustCompile(`\bask_user\b`), "提问"},
	// Runtime jargon.
	{regexp.MustCompile(`\bRun(?:Command|Scope|Mode|Phase)\b`), "任务设置"},
	{regexp.MustCompile(`(?i)\bcompletion gate\b`), "完成检查"},
	// Raw error codes such as EVIDENCE_HTML_MISSING (2+ underscore-joined caps).
	{regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+\b`), ""},
}

func redactInternalTerms(text string) string {
	if text == "" {
		return text
	}
	for _, rule := range internalTermReplacements {
		text = rule.pattern.ReplaceAllString(text, rule.replacement)
	}
	return text
}

type ToolPublicProjector struct {
	ProjectDir string
}

func (p ToolPublicProjector) Started(runID, callID, tool string, args map[string]any, planStepID string, decision *ToolDecision) (model.ToolStartedPayload, bool) {
	target := publicToolTarget(p.ProjectDir, tool, args)
	label, detail, ok := toolDisplay(p.ProjectDir, tool, args, true, ToolResult{})
	if !ok {
		return model.ToolStartedPayload{}, false
	}
	payload := model.ToolStartedPayload{
		PublicEventBase: publicBase(runID),
		CallID:          callID, Tool: tool, PlanStepID: planStepID,
		Target: target, Display: model.PublicDisplay{Label: label, Detail: detail},
	}
	if tool == "run_command" && decision != nil {
		payload.Command = &model.CommandProjection{Text: sanitizePublicText(decision.Command, 4096)}
	}
	return payload, true
}

func (p ToolPublicProjector) Completed(runID, callID, tool string, args map[string]any, result ToolResult) (model.ToolCompletedPayload, bool) {
	if result.Code == CodeDependencyFailed {
		return model.ToolCompletedPayload{}, false
	}
	label, detail, ok := toolDisplay(p.ProjectDir, tool, args, false, result)
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
		Target:  publicToolTarget(p.ProjectDir, tool, args),
		Display: model.PublicDisplay{Label: label, Detail: detail},
	}
	if tool == "run_command" && result.Command != nil {
		exitCode := result.Command.ExitCode
		durationMS := result.Command.DurationMS
		stdoutPreview, stderrPreview := publicCommandPreviews(result.Command)
		payload.Command = &model.CommandProjection{
			Text: sanitizePublicText(result.Command.Text, 4096), Status: result.Command.Status,
			ExitCode: &exitCode, DurationMS: &durationMS,
			OutputTruncated: result.Command.OutputTruncated,
			StdoutPreview:   stdoutPreview,
			StderrPreview:   stderrPreview,
		}
		payload.Status = result.Command.Status
	}
	if result.OK && len(result.ChangedTargets) > 0 {
		targets := publicChangedTargets(p.ProjectDir, result.ChangedTargets)
		if len(targets) > 0 {
			payload.Target = &targets[0]
		}
	}
	if !result.OK {
		agentErr := model.NewAgentError(publicErrorCode(result.Code), "tool_call", nil)
		payload.Error = agentErr.Public()
	}
	if tool == "render_slide" {
		payload.Preview = publicRenderPreview(runID, args, result)
	}
	for _, resource := range result.LoadedResources {
		if (resource.Kind != "component" && resource.Kind != "skill") ||
			resource.ID == "" || resource.Name == "" {
			continue
		}
		payload.Resources = append(payload.Resources, model.PublicLoadedResource{
			Kind: resource.Kind, ID: resource.ID, Name: sanitizePublicText(resource.Name, 200),
			OpenURL: sanitizePublicText(resource.OpenURL, 2048),
		})
	}
	return payload, true
}

func publicToolTarget(projectDir string, tool string, args map[string]any) *model.PublicTarget {
	if tool == "render_slide" {
		slideID := stringValue(args["slide_id"])
		if slideID != "" {
			target := &model.PublicTarget{Type: "slide", SlideID: slideID, Part: "html", DisplayName: slideDisplayName(slideID)}
			attachLocalOpenTarget(projectDir, target)
			return target
		}
	}
	if tool == "mutate_ppt" {
		op := stringValue(args["op"])
		slideID := stringValue(args["slide_id"])
		if strings.HasPrefix(op, "slide.") {
			part := "spec"
			if strings.HasPrefix(op, "slide.html.") {
				part = "html"
			}
			out := &model.PublicTarget{Type: "slide", SlideID: slideID, Part: part, DisplayName: runtimeSlideDisplayName(projectDir, slideID)}
			attachLocalOpenTarget(projectDir, out)
			return out
		}
		part := "outline"
		if strings.HasPrefix(op, "manifest.") {
			part = "manifest"
		}
		if strings.HasPrefix(op, "design.") {
			part = "design"
		}
		out := &model.PublicTarget{Type: "deck", Part: part}
		attachLocalOpenTarget(projectDir, out)
		return out
	}
	if tool == "run_command" {
		return nil
	}
	target, _ := args["resource"].(map[string]any)
	targetType := stringValue(target["kind"])
	if targetType == "" {
		targetType = stringValue(target["type"])
	}
	if targetType == "manifest" || targetType == "outline" || targetType == "design" {
		out := &model.PublicTarget{Type: "deck", Part: targetType}
		attachLocalOpenTarget(projectDir, out)
		return out
	}
	if targetType == "deck" {
		out := &model.PublicTarget{Type: "deck", Part: stringValue(target["part"])}
		attachLocalOpenTarget(projectDir, out)
		return out
	}
	if targetType == "slide" {
		slideID := stringValue(target["slide_id"])
		out := &model.PublicTarget{
			Type: "slide", SlideID: slideID, Part: stringValue(target["part"]), DisplayName: runtimeSlideDisplayName(projectDir, slideID),
		}
		attachLocalOpenTarget(projectDir, out)
		return out
	}
	return nil
}

func toolDisplay(projectDir string, tool string, args map[string]any, started bool, result ToolResult) (string, string, bool) {
	target := publicToolTarget(projectDir, tool, args)
	targetName := "内容"
	if target != nil && target.Type == "deck" && target.Part == "manifest" {
		targetName = "演示内容"
	} else if target != nil && target.Type == "deck" && target.Part == "outline" {
		targetName = "目录结构"
	} else if target != nil && target.Type == "deck" && target.Part == "design" {
		targetName = "视觉设计"
	} else if target != nil && target.Type == "slide" {
		pageName := target.DisplayName
		if pageName == "" {
			pageName = slideDisplayName(target.SlideID)
		}
		if target.Part == "spec" {
			targetName = pageName + "设计稿"
		} else {
			targetName = pageName + "幻灯片"
		}
	}
	switch tool {
	case "read_ppt":
		if started {
			return "读取" + targetName, "确认内容与设计约束", true
		}
		if result.OK {
			return "已读取" + targetName, targetDetail(target, "已获得所需内容"), true
		}
		return "读取" + targetName + "失败", publicToolError(result), true
	case "mutate_ppt":
		op := stringValue(args["op"])
		creating := strings.HasSuffix(op, ".write") || op == "outline.init" || op == "outline.insert" || op == "design.write"
		if started {
			if creating {
				return "创建" + targetName, "", true
			}
			return "更新" + targetName, "", true
		}
		if result.OK {
			if creating {
				return "已创建" + targetName, targetDetail(target, "内容已生成"), true
			}
			return "已更新" + targetName, targetDetail(target, "修改已完成"), true
		}
		return targetName + "操作失败", publicToolError(result), true
	case "render_slide":
		if started {
			return "检查" + targetName + "布局", "", true
		}
		if result.OK {
			return targetName + "渲染通过", renderDetail(result), true
		}
		return targetName + "渲染未通过", publicToolError(result), true
	case "run_command":
		if started {
			return "正在执行", "", true
		}
		if result.Command != nil {
			switch result.Command.Status {
			case "completed":
				stdoutPreview, _ := publicCommandPreviews(result.Command)
				return "已执行 1 条命令", stdoutPreview, true
			case "blocked":
				return "命令已被安全策略拦截", sanitizePublicText(result.Command.Reason, 300), true
			default:
				return "命令执行失败", sanitizePublicText(result.Command.Reason, 300), true
			}
		}
		return "命令执行失败", publicToolError(result), true
	case "load_component":
		if started {
			return "加载组件", "", true
		}
		if result.OK {
			return fmt.Sprintf("已加载 %d 个组件", len(result.LoadedResources)), "", true
		}
		return "组件加载失败", publicToolError(result), true
	case "load_skill":
		if started {
			return "加载技能", "", true
		}
		if result.OK {
			return fmt.Sprintf("已加载 %d 个技能", len(result.LoadedResources)), "", true
		}
		return "技能加载失败", publicToolError(result), true
	default:
		return "", "", false
	}
}

func publicCommandPreviews(command *CommandExecution) (string, string) {
	if command == nil {
		return "", ""
	}
	if command.Sensitive {
		stdout, stderr := "", ""
		if command.Stdout != "" {
			stdout = "[REDACTED]"
		}
		if command.Stderr != "" {
			stderr = "[REDACTED]"
		}
		return stdout, stderr
	}
	return sanitizeCommandPreview(command.Stdout, 8<<10), sanitizeCommandPreview(command.Stderr, 4<<10)
}

var terminalEscape = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

func sanitizeCommandPreview(value string, limit int) string {
	value = terminalEscape.ReplaceAllString(strings.ToValidUTF8(value, "\uFFFD"), "")
	value = sanitizePublicText(value, limit)
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func safeToolDetail(result ToolResult, fallback string) string {
	if !result.OK {
		return publicToolError(result)
	}
	text := sanitizePublicText(result.Summary, 100)
	normalized := strings.ToLower(strings.TrimSpace(text))
	if text == "" || internalToolSummary(normalized) {
		return fallback
	}
	return text
}

func internalToolSummary(value string) bool {
	switch value {
	case "resource staged", "resource read", "resource written", "resource edited atomically":
		return true
	default:
		return false
	}
}

func targetDetail(target *model.PublicTarget, fallback string) string {
	if target != nil && target.LocalPath != "" {
		return target.LocalPath
	}
	return fallback
}

func attachLocalOpenTarget(projectDir string, target *model.PublicTarget) {
	if target == nil || strings.TrimSpace(projectDir) == "" {
		return
	}
	rel := publicTargetRelativePath(*target)
	if rel == "" {
		return
	}
	localPath := filepath.Join(projectDir, rel)
	target.LocalPath = localPath
	target.OpenURL = (&url.URL{Scheme: "vscode", Host: "file", Path: filepath.ToSlash(localPath)}).String()
}

func publicTargetRelativePath(target model.PublicTarget) string {
	if target.Type == "deck" {
		switch target.Part {
		case "manifest":
			return "manifest.json"
		case "outline":
			return "outline.json"
		case "design":
			return "design.json"
		}
	}
	if target.Type == "slide" && target.SlideID != "" {
		switch target.Part {
		case "spec":
			return model.SlideSpecPath(target.SlideID)
		case "html":
			return model.SlideHTMLPath(target.SlideID)
		}
	}
	return ""
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

var trailingSlideNumber = regexp.MustCompile(`(\d+)$`)

func slideDisplayName(slideID string) string {
	match := trailingSlideNumber.FindStringSubmatch(slideID)
	if len(match) == 2 {
		if index, err := strconv.Atoi(match[1]); err == nil && index > 0 {
			return fmt.Sprintf("第 %d 页", index)
		}
	}
	if strings.TrimSpace(slideID) == "" {
		return "页面"
	}
	return "页面"
}

func runtimeSlideDisplayName(projectDir, slideID string) string {
	var outline spec.Outline
	var raw []byte
	var err error
	if session := ActiveRunSession(projectDir); session != nil {
		raw, err = session.ReadPath("outline.json")
	} else {
		raw, err = os.ReadFile(filepath.Join(projectDir, "outline.json"))
	}
	if err == nil && json.Unmarshal(raw, &outline) == nil {
		if ordinal, ok := spec.ResolveSlideOrdinal(outline, slideID); ok {
			return fmt.Sprintf("第 %d 页", ordinal)
		}
	}
	return slideDisplayName(slideID)
}

func newMessageID() string {
	return "msg_" + uuid.NewString()
}

func publicQuestion(runID, questionID string, args map[string]any) model.QuestionAskedPayload {
	questions := publicQuestionFields(args)
	return model.QuestionAskedPayload{
		PublicEventBase: publicBase(runID), QuestionID: questionID,
		Header: sanitizePublicText(stringValue(args["header"]), 24), Questions: questions,
	}
}

func publicQuestionFields(args map[string]any) []model.QuestionField {
	rawQuestions, _ := args["questions"].([]any)
	questions := []model.QuestionField{}
	seen := map[string]bool{}
	for index, raw := range rawQuestions {
		question, _ := raw.(map[string]any)
		id := sanitizePublicText(stringValue(question["id"]), 64)
		if id == "" {
			id = fmt.Sprintf("question-%d", index+1)
		}
		if seen[id] {
			id = fmt.Sprintf("%s-%d", id, index+1)
		}
		seen[id] = true
		field := publicQuestionField(question, id)
		if field.Title == "" {
			continue
		}
		questions = append(questions, field)
	}
	return questions
}

func publicQuestionField(args map[string]any, fallbackID string) model.QuestionField {
	options := publicQuestionOptions(args)
	allowCustom, _ := args["allow_custom"].(bool)
	if len(options) == 0 {
		allowCustom = true
	}
	title := sanitizePublicText(stringValue(args["title"]), 120)
	description := sanitizePublicText(stringValue(args["description"]), 260)
	return model.QuestionField{
		ID: fallbackID, Title: title, Description: description,
		Options: options, AllowCustom: allowCustom,
	}
}

func publicQuestionOptions(args map[string]any) []model.QuestionOption {
	options := []model.QuestionOption{}
	rawOptions, _ := args["options"].([]any)
	for index, raw := range rawOptions {
		if index >= 3 {
			break
		}
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
			Description: sanitizePublicText(stringValue(option["description"]), 220),
		})
	}
	return options
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

func safeFinalMessage(message string, mode model.RunMode, affected int) string {
	if text := sanitizePublicMarkdown(message, 0); text != "" {
		return text
	}
	if mode == model.ModeTalk || mode == model.ModeAsk {
		return "已完成本次分析。"
	}
	if mode == model.ModePlan {
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
