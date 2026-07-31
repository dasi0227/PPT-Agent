// design_director.go 是 v2 生成流水线的 Stage 1（设计总监节点，V2-AGENT-PIPELINE §3）。
// 一个 harness 子循环，工具集 {submit_design_spec, finish}，产出 design_spec 作为跨页设计契约。
package generate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
	"github.com/dasi0227/PPT-Agent/backend/internal/blueprint"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// runDesignDirector 执行 Stage 1：emit progress{design}，跑设计总监子循环，返回落盘的 design_spec。
// 失败（无法产出合法 design_spec）→ 返回错误，调用方据 V2-STOP-001 整体 failed，不进入后续阶段。
func (r *Runner) runDesignDirector(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, sandbox *tools.Sandbox, slides []model.Slide, themeName string) (*DesignSpec, error) {
	em.Emit(model.EventProgress, harness.ProgressPayload{
		Stage: "design", Current: 1, Total: 1, Message: "确定项目设计语言",
	})

	submit := NewSubmitDesignSpecTool(r.store, sandbox, r.params.ProjectID, r.params.RunID, r.clock, r.newID)

	titles := make([]string, 0, len(slides))
	for _, sl := range slides {
		titles = append(titles, pageTitle(sl))
	}
	dp := prompt.DesignParams{
		Topic:         r.designTopic(slides),
		Brief:         r.params.Brief,
		SlideCount:    len(slides),
		Language:      r.params.Language,
		OutlineTitles: titles,
		ThemeName:     r.userThemeName(themeName),
	}

	loop := harness.New(r.client, harness.Config{
		RunID:        r.params.RunID,
		Kind:         model.KindGenerate,
		Scope:        model.ScopeOverview,
		Mode:         model.ModeNormal,
		SystemPrompt: prompt.DesignSystem(),
		Instruction:  prompt.DesignUser(dp),
		Tools:        []tools.Tool{submit, tools.NewFinishTool()},
	})
	outcome := loop.Run(ctx, em, cp)

	// 优先用工具内存中的 spec；子循环 finish 但未提交则回退读盘。
	if submit.Spec() != nil {
		return submit.Spec(), nil
	}
	if spec, err := readDesignSpec(sandbox); err == nil {
		return spec, nil
	}
	if outcome.Status == harness.OutcomeCanceled {
		return nil, fmt.Errorf("canceled")
	}
	return nil, fmt.Errorf("设计总监未能产出合法 design_spec")
}

// designTopic 取项目主题：Brief 优先作提示，否则用首页（cover）标题近似。
func (r *Runner) designTopic(slides []model.Slide) string {
	if len(slides) > 0 {
		return pageTitle(slides[0])
	}
	return ""
}

// userThemeName 仅当用户显式指定主题时返回其名（用于设计总监"以主题为基底"）。
func (r *Runner) userThemeName(themeName string) string {
	if r.params.Theme != "" {
		return themeName
	}
	return ""
}

// readDesignSpec 从磁盘读取已落盘的 design_spec（单页重生成/回退路径）。
func readDesignSpec(sandbox *tools.Sandbox) (*DesignSpec, error) {
	raw, err := sandbox.Read("design/design-spec.json")
	if err != nil {
		return nil, err
	}
	var persisted blueprint.DesignSpec
	if err := json.Unmarshal(raw, &persisted); err != nil {
		return nil, err
	}
	if err := blueprint.ValidateDesignSpec(persisted); err != nil {
		return nil, err
	}
	spec := directorFromBlueprint(persisted)
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	return &spec, nil
}

func directorFromBlueprint(spec blueprint.DesignSpec) DesignSpec {
	out := DesignSpec{Palette: []DesignColor{}, Signature: spec.Signature}
	for i, hex := range spec.Palette {
		out.Palette = append(out.Palette, DesignColor{Name: fmt.Sprintf("color-%d", i+1), Hex: hex, Role: "design token"})
	}
	if raw, err := json.Marshal(spec.Typography); err == nil {
		_ = json.Unmarshal(raw, &out.Type)
	}
	if raw, err := json.Marshal(spec.LayoutSystem); err == nil {
		_ = json.Unmarshal(raw, &out.Layout)
	}
	if raw, err := json.Marshal(spec.Motion); err == nil {
		_ = json.Unmarshal(raw, &out.Motion)
	}
	return out
}

// briefFromSpec 把 design_spec 压缩为逐页注入的摘要（slide.gen@v2）。
func briefFromSpec(spec *DesignSpec, sj slidejson.SlideJSON, stepTitle, stepDetail string) *prompt.DesignBrief {
	if spec == nil {
		return nil
	}
	roles := make([]string, 0, len(spec.Palette))
	for _, c := range spec.Palette {
		roles = append(roles, fmt.Sprintf("%s→%s", c.Name, c.Role))
	}
	return &prompt.DesignBrief{
		Topic:         spec.Subject.Topic,
		Audience:      spec.Subject.Audience,
		PaletteRoles:  roles,
		DisplayFont:   spec.Type.Display.Family,
		BodyFont:      spec.Type.Body.Family,
		UtilityFont:   spec.Type.Utility.Family,
		LayoutConcept: spec.Layout.Concept,
		LayoutRhythm:  spec.Layout.Rhythm,
		Signature:     spec.Signature,
		MotionPolicy:  spec.Motion.Policy,
		StepTitle:     stepTitle,
		StepDetail:    stepDetail,
	}
}
