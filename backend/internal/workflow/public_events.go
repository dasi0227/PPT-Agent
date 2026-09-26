package workflow

import (
	"encoding/json"
	"fmt"
	"github.com/dasi0227/PPT-Agent/backend/internal/fileopen"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

func publicPlan(plan Plan, display ...model.PublicTextContext) model.PublicPlan {
	steps := make([]model.PublicPlanStep, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		steps = append(steps, model.PublicPlanStep{
			ID: step.ID, Title: model.PublicText(step.Title, display...), Status: string(step.Status), TargetSlideIDs: append([]string{}, step.TargetSlideIDs...),
		})
	}
	return model.PublicPlan{
		PlanID: plan.ID,
		Status: string(plan.Status), Title: model.PublicText(plan.Title, display...), Content: model.PublicText(plan.Content, display...), Steps: steps,
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

func milestoneText(title string, completed []PlanStep, display ...model.PublicTextContext) string {
	titles := make([]string, 0, len(completed))
	for _, step := range completed {
		if t := strings.TrimSpace(step.Title); t != "" {
			titles = append(titles, model.PublicText(t, display...))
		}
	}
	if len(titles) > 0 {
		return "已完成「" + strings.Join(titles, "」、「") + "」"
	}
	return model.PublicText(title, display...)
}

func sanitizePublicText(text string, _ int) string { return model.PublicText(text) }

type ToolPublicProjector struct {
	ProjectDir  string
	TextContext model.PublicTextContext
}

func (p ToolPublicProjector) Started(runID, callID, tool string, args map[string]any, planStepID string, decision *ToolDecision) (model.ToolStartedPayload, bool) {
	target := publicToolTarget(p.ProjectDir, tool, args)
	label, detail, ok := toolDisplay(p.ProjectDir, tool, args, true, ToolResult{}, p.TextContext)
	if !ok {
		return model.ToolStartedPayload{}, false
	}
	payload := model.ToolStartedPayload{
		PublicEventBase: publicBase(runID),
		CallID:          callID, Tool: tool, PlanStepID: planStepID,
		Target: target, Display: model.PublicDisplay{Label: label, Detail: detail},
	}
	if tool == "run_command" && decision != nil {
		payload.Command = &model.CommandProjection{Text: decision.Command}
	}
	return payload, true
}

func (p ToolPublicProjector) Completed(runID, callID, tool string, args map[string]any, result ToolResult) (model.ToolCompletedPayload, bool) {
	if result.Code == CodeDependencyFailed {
		return model.ToolCompletedPayload{}, false
	}
	label, detail, ok := toolDisplay(p.ProjectDir, tool, args, false, result, p.TextContext)
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
		stdoutPreview, stderrPreview := publicCommandPreviews(result.Command, p.TextContext)
		payload.Command = &model.CommandProjection{
			Text: result.Command.Text, Status: result.Command.Status,
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
			OpenURL: resource.OpenURL,
		})
	}
	return payload, true
}

func publicToolTarget(projectDir string, tool string, args map[string]any) *model.PublicTarget {
	if tool == "render_slide" {
		slideID := stringValue(args["slide_id"])
		if slideID != "" {
			target := &model.PublicTarget{Type: "slide", SlideID: slideID, Part: "html", DisplayName: runtimeSlideDisplayName(projectDir, slideID)}
			attachLocalOpenTarget(projectDir, target)
			return target
		}
	}
	if isResourceEditTool(tool) || tool == "read_resource" {
		resource := resourceForTool(tool, stringValue(args["slide_id"]))
		if tool == "read_resource" {
			var err error
			resource, err = parseResource(args)
			if err != nil {
				return nil
			}
		}
		out := &model.PublicTarget{Type: resource.Type, Part: resource.Part, SlideID: resource.SlideID}
		if resource.Type == "slide" {
			out.DisplayName = runtimeSlideDisplayName(projectDir, resource.SlideID)
		}
		attachLocalOpenTarget(projectDir, out)
		return out
	}

	if tool == "run_command" {
		return nil
	}
	return nil
}

func toolDisplay(projectDir string, tool string, args map[string]any, started bool, result ToolResult, display ...model.PublicTextContext) (string, string, bool) {
	target := publicToolTarget(projectDir, tool, args)
	targetName := "内容"
	if target != nil && target.Type == "deck" && target.Part == "manifest" {
		targetName = "内容要求"
	} else if target != nil && target.Type == "deck" && target.Part == "outline" {
		targetName = "目录结构"
	} else if target != nil && target.Type == "deck" && target.Part == "design" {
		targetName = "视觉要求"
	} else if target != nil && target.Type == "slide" {
		pageName := target.DisplayName
		if pageName == "" {
			pageName = slideDisplayName(target.SlideID)
		}
		if target.Part == "spec" {
			targetName = pageName + "规格要求"
		} else {
			targetName = pageName + "幻灯片"
		}
	}
	switch tool {
	case "read_resource":
		if started {
			return "读取" + targetName, "确认内容与设计约束", true
		}
		if result.OK {
			return "已读取" + targetName, targetDetail(target, "已获得所需内容"), true
		}
		return "读取" + targetName + "失败", publicToolError(result), true
	case "edit_manifest", "edit_design", "edit_spec", "init_outline", "arrange_outline", "write_html", "patch_html":
		creating := tool == "init_outline" || tool == "write_html"
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
		if creating {
			return "创建" + targetName + "失败", publicToolError(result), true
		}
		return "更新" + targetName + "失败", publicToolError(result), true
	case "render_slide":
		if started {
			return "正在渲染" + targetName, "", true
		}
		if result.OK {
			return "已渲染" + targetName, renderDetail(result), true
		}
		return "渲染" + strings.TrimSuffix(targetName, "幻灯片") + "失败", publicToolError(result), true
	case "run_command":
		if started {
			return "正在执行", "", true
		}
		if result.Command != nil {
			switch result.Command.Status {
			case "completed":
				stdoutPreview, _ := publicCommandPreviews(result.Command, display...)
				return "已执行 1 条命令", stdoutPreview, true
			case "blocked":
				return "命令已被安全策略拦截", model.PublicText(result.Command.Reason, display...), true
			default:
				return "命令执行失败", model.PublicText(result.Command.Reason, display...), true
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

func publicCommandPreviews(command *CommandExecution, display ...model.PublicTextContext) (string, string) {
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
	return sanitizeCommandPreview(command.Stdout, 8<<10, display...), sanitizeCommandPreview(command.Stderr, 4<<10, display...)
}

var terminalEscape = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

func sanitizeCommandPreview(value string, limit int, display ...model.PublicTextContext) string {
	value = terminalEscape.ReplaceAllString(strings.ToValidUTF8(value, "\uFFFD"), "")
	value = model.PublicText(value, display...)
	if len(value) <= limit {
		return value
	}
	return strings.ToValidUTF8(value[:limit], "")
}

func targetDetail(_ *model.PublicTarget, fallback string) string { return fallback }

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
	target.OpenURL = fileopen.OpenURL(localPath)
}

func publicTargetRelativePath(target model.PublicTarget) string {
	if target.Type == "slide" && target.Part == "html" && target.SlideID != "" {
		return model.SlideHTMLPath(target.SlideID)
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

func slideDisplayName(_ string) string { return "相关页面" }

func runtimeSlideDisplayName(projectDir, slideID string) string {
	var outline spec.Outline
	var raw []byte
	var err error
	if session := ActiveRunSession(projectDir); session != nil {
		raw, err = session.ReadPath(".outline.json")
	} else {
		raw, err = os.ReadFile(filepath.Join(projectDir, ".outline.json"))
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

func publicQuestion(runID, questionID string, args map[string]any, display ...model.PublicTextContext) model.QuestionAskedPayload {
	questions := publicQuestionFields(args, display...)
	return model.QuestionAskedPayload{
		PublicEventBase: publicBase(runID), QuestionID: questionID,
		Header: model.PublicText(stringValue(args["header"]), display...), Questions: questions,
	}
}

func publicQuestionFields(args map[string]any, display ...model.PublicTextContext) []model.QuestionField {
	rawQuestions, _ := args["questions"].([]any)
	questions := []model.QuestionField{}
	seen := map[string]bool{}
	for index, raw := range rawQuestions {
		question, _ := raw.(map[string]any)
		id := strings.TrimSpace(stringValue(question["id"]))
		if id == "" {
			id = fmt.Sprintf("question-%d", index+1)
		}
		if seen[id] {
			id = fmt.Sprintf("%s-%d", id, index+1)
		}
		seen[id] = true
		field := publicQuestionField(question, id, display...)
		if field.Title == "" {
			continue
		}
		questions = append(questions, field)
	}
	return questions
}

func publicQuestionField(args map[string]any, fallbackID string, display ...model.PublicTextContext) model.QuestionField {
	options := publicQuestionOptions(args, display...)
	allowCustom, _ := args["allow_custom"].(bool)
	if len(options) == 0 {
		allowCustom = true
	}
	title := model.PublicText(stringValue(args["title"]), display...)
	description := model.PublicText(stringValue(args["description"]), display...)
	return model.QuestionField{
		ID: fallbackID, Title: title, Description: description,
		Options: options, AllowCustom: allowCustom,
	}
}

func publicQuestionOptions(args map[string]any, display ...model.PublicTextContext) []model.QuestionOption {
	options := []model.QuestionOption{}
	rawOptions, _ := args["options"].([]any)
	for index, raw := range rawOptions {
		if index >= 3 {
			break
		}
		option, _ := raw.(map[string]any)
		label := model.PublicText(stringValue(option["label"]), display...)
		if label == "" {
			continue
		}
		id := strings.TrimSpace(stringValue(option["id"]))
		if id == "" {
			id = fmt.Sprintf("option-%d", index+1)
		}
		options = append(options, model.QuestionOption{
			ID: id, Label: label,
			Description: model.PublicText(stringValue(option["description"]), display...),
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

func safeFinalMessage(message string, mode model.RunMode, affected int, contexts ...model.PublicTextContext) string {
	if text := model.PublicText(message, contexts...); text != "" {
		return text
	}
	if mode == model.ModeChat || mode == model.ModeGrill {
		return "已完成本次分析。"
	}
	if mode == model.ModePlan {
		return "已完成本次计划。"
	}
	if affected > 0 {
		return fmt.Sprintf("已保存 %d 项修改。", affected)
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
