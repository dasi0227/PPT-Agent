package outline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// 中间正文页数量默认区间（未指定 slide_count 时，AC-OUTLINE-002）。
const (
	defaultMinBody = 5
	defaultMaxBody = 20
)

// SubmitOutlineTool 接收 LLM 提交的 slide-json[]，校验后落库并产版本（M2 核心工具）。
// 只产 slide-json（内容与意图），绝不产 html（两阶段边界）。
type SubmitOutlineTool struct {
	store      Store
	sandbox    *tools.Sandbox
	projectID  string
	runID      string
	slideCount int // 用户指定的总页数；0 表示未指定
	clock      func() int64
	newID      func() string
}

// NewSubmitOutlineTool 构造工具。clock/newID 可注入以便测试确定性。
func NewSubmitOutlineTool(store Store, sandbox *tools.Sandbox, projectID, runID string, slideCount int, clock func() int64, newID func() string) *SubmitOutlineTool {
	if clock == nil {
		clock = func() int64 { return 0 }
	}
	if newID == nil {
		newID = uuid.NewString
	}
	return &SubmitOutlineTool{
		store: store, sandbox: sandbox, projectID: projectID, runID: runID,
		slideCount: slideCount, clock: clock, newID: newID,
	}
}

func (t *SubmitOutlineTool) Name() string          { return "submit_outline" }
func (t *SubmitOutlineTool) Class() tools.Class    { return tools.ClassWrite }
func (t *SubmitOutlineTool) Scopes() []model.Scope { return nil }

func (t *SubmitOutlineTool) Description() string {
	return "提交整份演示文稿大纲（slide-json 数组）。系统会校验 schema 与结构约束、落库并创建版本。" +
		"不要提供 id/idx（由系统回填）。校验失败会返回可操作错误，请修正后重新提交。"
}

func (t *SubmitOutlineTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"slides"},
		"properties": map[string]any{
			"slides": map[string]any{
				"type":        "array",
				"minItems":    2,
				"description": "slide-json 数组；首页 layout=cover，末页 layout∈{thanks,cta}",
				"items": map[string]any{
					"type":     "object",
					"required": []any{"layout", "title"},
					"properties": map[string]any{
						"layout":         map[string]any{"type": "string", "enum": toAnySlice(slidejson.LayoutEnum())},
						"title":          map[string]any{"type": "string"},
						"subtitle":       map[string]any{"type": "string"},
						"bullets":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"content_intent": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}

func (t *SubmitOutlineTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	slides, err := parseSlides(args["slides"])
	if err != nil {
		return fail(err.Error()), nil
	}
	if len(slides) < 2 {
		return fail("大纲至少需要封面页与结尾页两页"), nil
	}

	// 服务端权威回填 id/idx，随后逐页 schema 校验（SPEC-OUTLINE-001）。
	for i := range slides {
		slides[i].ID = t.newID()
		slides[i].Idx = i
		if slides[i].Bullets == nil && strings.TrimSpace(slides[i].ContentIntent) == "" {
			return fail(fmt.Sprintf("第 %d 页缺少 bullets 或 content_intent（SPEC-OUTLINE-003）", i+1)), nil
		}
		if err := slidejson.Validate(slides[i]); err != nil {
			return fail(fmt.Sprintf("第 %d 页 schema 校验失败：%v", i+1, err)), nil
		}
	}

	// 结构约束：首尾页 + 页数（AC-OUTLINE-002）。
	if err := t.checkStructure(slides); err != nil {
		return fail(err.Error()), nil
	}

	// 落盘 slide.json + project.json，再落库替换 slides 并产版本。
	metas, err := t.writeFiles(slides)
	if err != nil {
		return fail(fmt.Sprintf("写入文件失败：%v", err)), nil
	}
	if err := t.store.ReplaceSlides(ctx, t.projectID, metas); err != nil {
		return tools.Result{}, err
	}
	versionNo, err := t.createProjectVersion(ctx, slides)
	if err != nil {
		return tools.Result{}, err
	}
	// project 保持 draft：outline 阶段仅有 slide-json，HTML 未生成，未达 ready 语义
	// （data-model.md: ready = 全部 slides 已有 html）。M3 slide 生成完成后再转 generating→ready。

	return tools.Result{
		OK:          true,
		Observation: fmt.Sprintf("大纲已提交：%d 页，已落库并创建 project 版本 v%d", len(slides), versionNo),
		Artifact:    &tools.Artifact{Type: "outline", Ref: "project.json"},
	}, nil
}

// checkStructure 校验首尾页与页数约束（AC-OUTLINE-002）。
func (t *SubmitOutlineTool) checkStructure(slides []slidejson.SlideJSON) error {
	if slides[0].Layout != "cover" {
		return fmt.Errorf("首页 layout 必须为 cover，当前为 %q", slides[0].Layout)
	}
	last := slides[len(slides)-1].Layout
	if last != "thanks" && last != "cta" {
		return fmt.Errorf("末页 layout 必须为 thanks 或 cta，当前为 %q", last)
	}
	if t.slideCount > 0 {
		if len(slides) != t.slideCount {
			return fmt.Errorf("总页数必须为 %d，当前为 %d", t.slideCount, len(slides))
		}
		return nil
	}
	body := len(slides) - 2
	if body < defaultMinBody || body > defaultMaxBody {
		return fmt.Errorf("中间正文页数应在 %d–%d 之间，当前为 %d", defaultMinBody, defaultMaxBody, body)
	}
	return nil
}

// writeFiles 落盘每页 slide.json 与 project.json，返回 slide 元数据（DATA-FS-LAYOUT）。
func (t *SubmitOutlineTool) writeFiles(slides []slidejson.SlideJSON) ([]model.Slide, error) {
	metas := make([]model.Slide, len(slides))
	for i, s := range slides {
		dir := fmt.Sprintf("slides/%03d", s.Idx)
		jsonPath := dir + "/slide.json"
		htmlPath := dir + "/index.html" // 路径登记，html 由 M3 生成
		raw, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := t.sandbox.Write(jsonPath, raw); err != nil {
			return nil, err
		}
		metas[i] = model.Slide{
			ID: s.ID, ProjectID: t.projectID, Idx: s.Idx, Layout: s.Layout, Title: s.Title,
			JSONPath: jsonPath, HTMLPath: htmlPath, CurrentVersion: 0,
		}
	}
	projectDoc := map[string]any{
		"project_id":  t.projectID,
		"slide_count": len(slides),
		"slides":      slides,
	}
	praw, err := json.MarshalIndent(projectDoc, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := t.sandbox.Write("project.json", praw); err != nil {
		return nil, err
	}
	return metas, nil
}

// createProjectVersion 快照 project.json 并登记版本（version 0 起，SPEC-OUTLINE-008）。
func (t *SubmitOutlineTool) createProjectVersion(ctx context.Context, slides []slidejson.SlideJSON) (int, error) {
	no, err := t.store.NextVersionNo(ctx, "project", t.projectID)
	if err != nil {
		return 0, err
	}
	snapshotPath := fmt.Sprintf("versions/project/v%d.json", no)
	raw, err := json.MarshalIndent(map[string]any{"slides": slides}, "", "  ")
	if err != nil {
		return 0, err
	}
	if err := t.sandbox.Write(snapshotPath, raw); err != nil {
		return 0, err
	}
	v := model.Version{
		ID: t.newID(), TargetType: "project", TargetID: t.projectID, VersionNo: no,
		SnapshotPath: snapshotPath, RunID: t.runID, CreatedAt: t.clock(),
	}
	if err := t.store.CreateVersion(ctx, v); err != nil {
		return 0, err
	}
	return no, nil
}

// parseSlides 把工具参数解析为 SlideJSON（忽略 LLM 误填的 id/idx，由服务端回填）。
func parseSlides(v any) ([]slidejson.SlideJSON, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("参数错误：slides 无法序列化")
	}
	var list []slidejson.SlideJSON
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("参数错误：slides 必须为 slide-json 数组")
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("参数错误：slides 为空")
	}
	return list, nil
}

func toAnySlice(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func fail(reason string) tools.Result {
	return tools.Result{OK: false, Observation: reason}
}
