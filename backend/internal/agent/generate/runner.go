// Package generate 是真实的 slide 生成 agent（slide-json[] + 主题 → slide html）。
// 复用 M1 的 run+harness+llm：父层做确定性 Go 编排（分页/进度/落状态/公共层），
// 每页派发一个独立 harness.Loop 子代理（独立上下文，ARCH-HARNESS-005）。编辑不走子代理（那是 M4）。
package generate

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

// Params 是构造一次生成 Run 所需的输入。
type Params struct {
	RunID     string
	ProjectID string
	WorkDir   string // project work_dir（sandbox 根）
	WorkRoot  string // 全局 work_root（_assets 所在）；空时按 WorkDir 父目录兜底
	Theme     string // 所选主题 id；空则回退 project.Theme→首个 preset
	PageIndex *int   // 非空=单页重生成，仅动该页（AC-GEN-009）；空=整套生成
	Brief     string // 设计总监上下文：项目简报（可空）
	Language  string // 设计总监上下文：语言（zh/en，可空）
}

type resolvedTheme struct {
	Name  string
	Asset *model.Asset
	Seed  bool
}

// Runner 用逐页子代理跑「slide-json[] + 主题 → slide html」，满足 run.Runner。
type Runner struct {
	client llm.Client
	store  Store
	params Params
	clock  func() int64
	newID  func() string
}

// NewRunner 构造生成 runner。clock/newID 可注入以便测试确定性；nil 用默认。
func NewRunner(client llm.Client, store Store, p Params, clock func() int64, newID func() string) *Runner {
	if clock == nil {
		clock = func() int64 { return time.Now().Unix() }
	}
	if newID == nil {
		newID = uuid.NewString
	}
	return &Runner{client: client, store: store, params: p, clock: clock, newID: newID}
}

func (r *Runner) Run(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, _ run.Prompter) harness.Outcome {
	sandbox, err := tools.NewSandbox(r.params.WorkDir)
	if err != nil {
		return r.errOut(em, "INTERNAL", err.Error())
	}

	slides, err := r.store.ListSlides(ctx, r.params.ProjectID)
	if err != nil {
		return r.errOut(em, "INTERNAL", err.Error())
	}
	if len(slides) == 0 {
		return r.errOut(em, "BAD_STATE", "无可生成的 slide（请先完成大纲）")
	}

	theme, err := r.resolveTheme(ctx)
	if err != nil {
		return r.errOut(em, "BAD_STATE", err.Error())
	}

	// 目标页集合：单页重生成只动该页，不碰公共层与别页（AC-GEN-009）。
	single := r.params.PageIndex != nil
	var spec *DesignSpec
	if single {
		idx := *r.params.PageIndex
		if idx < 0 || idx >= len(slides) {
			return r.errOut(em, "BAD_REQUEST", fmt.Sprintf("页号越界：%d（共 %d 页）", idx, len(slides)))
		}
		// 单页重生成走精简路径：读现有 design_spec（若有），不重跑设计总监（§7）。
		spec, _ = readDesignSpec(sandbox)
	} else {
		// 整套生成 Stage 1：设计总监节点产出 design_spec（V2-AGENT-PIPELINE §3）。
		spec, err = r.runDesignDirector(ctx, em, cp, sandbox, slides, theme.Name)
		if err != nil {
			// V2-STOP-001：无法产出合法 design_spec → 整体 failed，不进入后续阶段。
			return r.errOut(em, "DESIGN_FAILED", err.Error())
		}
		// 整套生成：写公共层（tokens 由 design_spec 生成；用户指定主题时以主题为基底叠加）。
		if err := r.writeCommon(sandbox, theme, spec); err != nil {
			return r.errOut(em, "INTERNAL", err.Error())
		}
		_ = r.store.SetProjectStatus(ctx, r.params.ProjectID, "generating")
	}

	targets := slides
	if single {
		targets = slides[*r.params.PageIndex : *r.params.PageIndex+1]
	}

	// 整套生成：先发一次完整 plan（V2-PLAN-001），逐页再用 plan.update 推进。
	// 单页重生成走精简路径，不发跨页 plan（V2-AGENT-PIPELINE §7）。
	plan := buildPlan(r.params.RunID, slides)
	if !single {
		em.Emit(model.EventPlan, plan)
	}

	total := len(targets)
	var warnings []Warning // 交付告警累积（页失败/修复超限），不阻塞整体 done（V2-STOP-002）。
	for i, sl := range targets {
		if ctx.Err() != nil {
			// V2-STOP-003：取消 → 安全终止，保留已落盘页与 design_spec。
			return harness.Outcome{Status: harness.OutcomeCanceled, Code: harness.CodeCanceled, Message: "run canceled"}
		}
		// 逐页 progress（AC-GEN-006）：stage=page，current/total 为业务页进度。
		em.Emit(model.EventProgress, harness.ProgressPayload{
			Stage: "page", Current: i + 1, Total: total,
			Message: fmt.Sprintf("生成第 %d 页（%s）", sl.Idx, sl.Layout),
		})
		if !single {
			em.Emit(model.EventPlanUpdate, harness.PlanUpdatePayload{
				ID: plan.ID, StepID: pageStepID(sl.Idx), Status: planStatusInProgress,
			})
		}

		sj, err := r.loadSlideJSON(sandbox, sl)
		if err != nil {
			return r.errOut(em, "INTERNAL", err.Error())
		}

		// 注入 design_spec 摘要，保证跨页设计语言一致（slide.gen@v2）。
		brief := r.pageBrief(plan, spec, sj)
		outcome := r.generatePage(ctx, em, cp, sandbox, sj, theme.Name, brief, nil)
		if outcome.Status == harness.OutcomeCanceled {
			return outcome // 取消优先：安全终止（V2-STOP-003）。
		}
		if outcome.Status != harness.OutcomeFinished {
			// 单页精简路径：某页失败即整体失败（保留已落盘页），不进入跨页流程。
			if single {
				return outcome
			}
			// 整套生成：某页失败不阻塞整体 done，标 failed 入 warnings，继续其余页（V2-STOP-002）。
			em.Emit(model.EventPlanUpdate, harness.PlanUpdatePayload{
				ID: plan.ID, StepID: pageStepID(sl.Idx), Status: planStatusFailed,
			})
			warnings = append(warnings, Warning{PageIndex: sl.Idx, Code: warnFixExceeded, Message: outcome.Message})
			continue
		}
		if !single {
			em.Emit(model.EventPlanUpdate, harness.PlanUpdatePayload{
				ID: plan.ID, StepID: pageStepID(sl.Idx), Status: planStatusCompleted,
			})
		}
	}

	// 单页重生成走精简路径：不做跨页校验/结构化交付（§7）。
	if single {
		return harness.Outcome{
			Status:  harness.OutcomeFinished,
			Summary: fmt.Sprintf("已重生成第 %d 页 slide html（主题 %s）", targets[0].Idx, theme.Name),
		}
	}

	// Stage 4：全局校验 + 有限修复子循环（跨页一致性）。
	em.Emit(model.EventPlanUpdate, harness.PlanUpdatePayload{ID: plan.ID, StepID: planStepValidate, Status: planStatusInProgress})
	fixWarnings, canceled := r.validateAndFix(ctx, em, cp, sandbox, plan, slides, spec, theme.Name, failedPages(warnings))
	if canceled {
		return harness.Outcome{Status: harness.OutcomeCanceled, Code: harness.CodeCanceled, Message: "run canceled"}
	}
	warnings = append(warnings, fixWarnings...)
	em.Emit(model.EventPlanUpdate, harness.PlanUpdatePayload{ID: plan.ID, StepID: planStepValidate, Status: planStatusCompleted})

	// Stage 5：结构化交付（done.result）。
	em.Emit(model.EventPlanUpdate, harness.PlanUpdatePayload{ID: plan.ID, StepID: planStepDeliver, Status: planStatusInProgress})
	_ = r.store.SetProjectStatus(ctx, r.params.ProjectID, "ready")
	em.Emit(model.EventPlanUpdate, harness.PlanUpdatePayload{ID: plan.ID, StepID: planStepDeliver, Status: planStatusCompleted})

	return harness.Outcome{
		Status:  harness.OutcomeFinished,
		Summary: fmt.Sprintf("已生成 %d 页 slide html（主题 %s）", total, theme.Name),
		Result:  r.deliverResult(slides, spec, theme, warnings),
	}
}

// pageBrief 组装某页注入的 design_spec 摘要（含该页在计划中的角色）。
func (r *Runner) pageBrief(plan harness.PlanPayload, spec *DesignSpec, sj slidejson.SlideJSON) *prompt.DesignBrief {
	if spec == nil {
		return nil
	}
	step := stepByID(plan, pageStepID(sj.Idx))
	return briefFromSpec(spec, sj, step.Title, step.Detail)
}

// validateAndFix 是 Stage 4：逐页读盘跨页校验，不合格页触发有限修复子循环（slide.fix@v1）。
// 返回修复超限的告警与是否被取消。emit progress{validate}；修复步复用 plan.update(pageStep)。
func (r *Runner) validateAndFix(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, sandbox *tools.Sandbox, plan harness.PlanPayload, slides []model.Slide, spec *DesignSpec, themeName string, skip map[int]bool) ([]Warning, bool) {
	var warnings []Warning
	total := len(slides)
	for i, sl := range slides {
		if ctx.Err() != nil {
			return warnings, true
		}
		// progress{validate}：current=已校验页，total=总页数（V2-CONTRACTS §2.2）。
		em.Emit(model.EventProgress, harness.ProgressPayload{
			Stage: "validate", Current: i + 1, Total: total,
			Message: fmt.Sprintf("校验第 %d 页", sl.Idx+1),
		})
		if skip[sl.Idx] {
			continue // Stage 3 已失败并计入 warnings，不重复校验。
		}
		rel := model.SlideHTMLPath(sl.ID)
		html, err := sandbox.Read(rel)
		if err != nil {
			warnings = append(warnings, Warning{PageIndex: sl.Idx, Code: warnFixExceeded, Message: "校验阶段读取页面失败：" + err.Error()})
			continue
		}
		issues := crossPageLint(html, spec)
		if len(issues) == 0 {
			continue
		}
		// 有限修复子循环：最多 maxFixRounds 次，仍不合格则该页 failed 入 warnings（V2-STOP-002）。
		if fixed, canceled := r.fixPage(ctx, em, cp, sandbox, plan, sl, spec, themeName, issues); canceled {
			return warnings, true
		} else if !fixed {
			em.Emit(model.EventPlanUpdate, harness.PlanUpdatePayload{ID: plan.ID, StepID: pageStepID(sl.Idx), Status: planStatusFailed})
			warnings = append(warnings, Warning{
				PageIndex: sl.Idx, Code: warnFixExceeded,
				Message: fmt.Sprintf("第 %d 页经 %d 轮修复仍不合规：%s", sl.Idx+1, maxFixRounds, strings.Join(issues, "；")),
			})
		}
	}
	return warnings, false
}

// fixPage 对单页执行有限次修复子循环：回灌 lint 错误 → 子代理重写 → 重新校验。
// 返回是否修复成功、是否被取消。
func (r *Runner) fixPage(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, sandbox *tools.Sandbox, plan harness.PlanPayload, sl model.Slide, spec *DesignSpec, themeName string, issues []string) (bool, bool) {
	rel := model.SlideHTMLPath(sl.ID)
	for round := 0; round < maxFixRounds; round++ {
		if ctx.Err() != nil {
			return false, true
		}
		// 修复步：复用该页 plan.update(in_progress) 表征正在修复。
		em.Emit(model.EventPlanUpdate, harness.PlanUpdatePayload{
			ID: plan.ID, StepID: pageStepID(sl.Idx), Status: planStatusInProgress,
			Detail: fmt.Sprintf("修复第 %d 轮", round+1),
		})
		sj, err := r.loadSlideJSON(sandbox, sl)
		if err != nil {
			return false, false
		}
		brief := r.pageBrief(plan, spec, sj)
		outcome := r.generatePage(ctx, em, cp, sandbox, sj, themeName, brief, issues)
		if outcome.Status == harness.OutcomeCanceled {
			return false, true
		}
		if outcome.Status != harness.OutcomeFinished {
			continue // 本轮修复未产出，进入下一轮（受 maxFixRounds 限制）。
		}
		html, err := sandbox.Read(rel)
		if err != nil {
			continue
		}
		issues = crossPageLint(html, spec)
		if len(issues) == 0 {
			em.Emit(model.EventPlanUpdate, harness.PlanUpdatePayload{ID: plan.ID, StepID: pageStepID(sl.Idx), Status: planStatusCompleted})
			return true, false
		}
	}
	return false, false
}

// deliverResult 组装 Stage 5 结构化交付载体（V2-AGENT-PIPELINE §8.2）。
func (r *Runner) deliverResult(slides []model.Slide, spec *DesignSpec, theme resolvedTheme, warnings []Warning) map[string]any {
	if warnings == nil {
		warnings = []Warning{}
	}
	themeLabel := theme.Name
	if r.params.Theme == "" {
		themeLabel = "project-custom" // 未指定主题：tokens 由 design_spec 生成。
	}
	result := map[string]any{
		"project_id":  r.params.ProjectID,
		"slide_count": len(slides),
		"theme":       themeLabel,
		"warnings":    warnings,
	}
	if spec != nil {
		result["design_spec_ref"] = "design/design-spec.json"
		result["signature"] = spec.Signature
	}
	return result
}

// failedPages 把已产生的告警转为页号集合，供 Stage 4 跳过已失败页。
func failedPages(warnings []Warning) map[int]bool {
	if len(warnings) == 0 {
		return nil
	}
	m := make(map[int]bool, len(warnings))
	for _, w := range warnings {
		m[w.PageIndex] = true
	}
	return m
}

// generatePage 为单页派发一个独立 harness 子代理（独立上下文 ARCH-HARNESS-005）。
// fixErrors 非空时进入修复子循环（slide.fix@v1），只修列出的不合规项。
func (r *Runner) generatePage(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, sandbox *tools.Sandbox, sj slidejson.SlideJSON, themeName string, brief *prompt.DesignBrief, fixErrors []string) harness.Outcome {
	writeTool := NewWriteSlideTool(r.store, sandbox, r.params.ProjectID, r.params.RunID, sj.Idx, sj.ID, r.clock, r.newID)

	pp := prompt.SlideParams{
		Slide:     sj,
		Theme:     themeName,
		TokensRel: "../../common/tokens.css",
		BaseRel:   "../../common/base.css",
		Layouts:   slidejson.LayoutEnum(),
		Design:    brief,
		FixErrors: fixErrors,
	}

	loop := harness.New(r.client, harness.Config{
		RunID:        r.params.RunID,
		Kind:         model.KindGenerate,
		Scope:        model.ScopeCurrent,
		Mode:         model.ModeNormal,
		SystemPrompt: prompt.SlideSystem(pp),
		Instruction:  prompt.SlideUser(pp),
		Tools:        []tools.Tool{writeTool, NewValidateSlideTool(), tools.NewFinishTool()},
	})
	outcome := loop.Run(ctx, em, cp)
	// 防守：LLM 声称完成但未真正写入该页 → 视为失败（避免空页混过 AC-GEN-001）。
	if outcome.Status == harness.OutcomeFinished && !writeTool.Written() {
		return harness.Outcome{
			Status: harness.OutcomeLLMError, Code: "NO_OUTPUT",
			Message: fmt.Sprintf("第 %d 页未产出 html", sj.Idx),
		}
	}
	return outcome
}

// resolveTheme 解析所选主题 id/name：仓库资产优先，seed 仅作冷启动/兜底。
func (r *Runner) resolveTheme(ctx context.Context) (resolvedTheme, error) {
	themes, err := r.store.ListAssets(ctx, "theme")
	if err != nil {
		return resolvedTheme{}, err
	}
	if r.params.Theme != "" {
		for _, a := range themes {
			if a.ID == r.params.Theme || a.Name == r.params.Theme {
				aa := a
				return resolvedTheme{Name: a.Name, Asset: &aa}, nil
			}
		}
		if _, err := asset.ReadSeedFile(path.Join("assets", "themes", r.params.Theme, "tokens.css")); err == nil {
			return resolvedTheme{Name: r.params.Theme, Seed: true}, nil
		}
		return resolvedTheme{}, fmt.Errorf("主题资产不存在：%s", r.params.Theme)
	}
	if len(themes) > 0 {
		aa := themes[0]
		return resolvedTheme{Name: aa.Name, Asset: &aa}, nil
	}
	if _, err := asset.ReadSeedFile(path.Join("assets", "themes", "swiss-modern", "tokens.css")); err == nil {
		return resolvedTheme{Name: "swiss-modern", Seed: true}, nil
	}
	return resolvedTheme{}, fmt.Errorf("无可用主题资产（seed 未载入？）")
}

// writeCommon 写公共层：base.css=seed 基座；tokens.css 依主题协同策略生成（V2-AGENT-PIPELINE §3.4/§6）。
//   - 用户未指定主题：由 design_spec 生成项目专属 tokens（自由创作）。
//   - 用户指定主题资产：以主题 tokens 为基底，仅叠加字体角色等 signature 相关增量，不覆盖主色。
func (r *Runner) writeCommon(sandbox *tools.Sandbox, theme resolvedTheme, spec *DesignSpec) error {
	tokens, err := r.resolveTokens(theme, spec)
	if err != nil {
		return err
	}
	if err := sandbox.Write("common/tokens.css", tokens); err != nil {
		return err
	}
	base, err := asset.ReadSeedFile(path.Join("common", "base.css"))
	if err != nil {
		return fmt.Errorf("读取 base.css 失败：%w", err)
	}
	return sandbox.Write("common/base.css", base)
}

// resolveTokens 按主题协同策略产出 tokens.css 内容。
func (r *Runner) resolveTokens(theme resolvedTheme, spec *DesignSpec) ([]byte, error) {
	// 用户未显式指定主题：优先用 design_spec 生成项目专属 tokens。
	if r.params.Theme == "" && spec != nil {
		return tokensFromSpec(*spec), nil
	}
	// 用户指定了主题：以主题 tokens 为基底，叠加 spec 的字体角色（不覆盖主色）。
	base, err := r.readThemeTokens(theme)
	if err != nil {
		return nil, err
	}
	if spec != nil {
		base = tokensWithSpecOverlay(base, *spec)
	}
	return base, nil
}

func (r *Runner) readThemeTokens(theme resolvedTheme) ([]byte, error) {
	if theme.Asset == nil || theme.Seed {
		tokens, err := asset.ReadSeedFile(path.Join("assets", "themes", theme.Name, "tokens.css"))
		if err != nil {
			return nil, fmt.Errorf("读取主题 %s tokens 失败：%w", theme.Name, err)
		}
		return tokens, nil
	}
	workRoot := r.params.WorkRoot
	if workRoot == "" {
		workRoot = filepath.Dir(r.params.WorkDir)
	}
	assetSandbox, err := tools.NewSandbox(workRoot)
	if err != nil {
		return nil, err
	}
	manifestRaw, err := assetSandbox.Read(theme.Asset.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("读取主题 manifest 失败：%w", err)
	}
	if err := asset.ValidateManifestRaw(manifestRaw); err != nil {
		return nil, fmt.Errorf("theme manifest 校验失败：%w", err)
	}
	m, err := asset.Parse(manifestRaw)
	if err != nil {
		return nil, err
	}
	if m.Kind != asset.KindTheme {
		return nil, fmt.Errorf("资产 %s 不是 theme", theme.Name)
	}
	tokens, err := assetSandbox.Read(path.Join(theme.Asset.Dir, m.Assets.Tokens))
	if err != nil {
		return nil, fmt.Errorf("读取主题 %s tokens 失败：%w", theme.Name, err)
	}
	if miss := asset.ValidateThemeTokens(tokens); len(miss) != 0 {
		return nil, fmt.Errorf("theme 缺少必需 token：%v", miss)
	}
	return tokens, nil
}

// loadSlideJSON 从磁盘读取该页 slide.json（大纲阶段已落盘）。
func (r *Runner) loadSlideJSON(sandbox *tools.Sandbox, sl model.Slide) (slidejson.SlideJSON, error) {
	raw, err := sandbox.Read(model.SlideJSONPath(sl.ID))
	if err != nil {
		// 回退：磁盘无 slide.json 时用 store 元数据最小重建（layout/title 足够生成）。
		return slidejson.SlideJSON{ID: sl.ID, Idx: sl.Idx, Layout: sl.Layout, Title: sl.Title}, nil
	}
	sj, err := slidejson.Parse(raw)
	if err != nil {
		return slidejson.SlideJSON{}, err
	}
	// 磁盘 slide.json 未必带稳定 id，用 store 元数据回填，保证路径拼接一致。
	sj.ID = sl.ID
	return sj, nil
}

func (r *Runner) errOut(em harness.Emitter, code, msg string) harness.Outcome {
	em.Emit(model.EventError, harness.ErrorPayload{Code: code, Message: msg})
	return harness.Outcome{Status: harness.OutcomeLLMError, Code: code, Message: msg}
}
