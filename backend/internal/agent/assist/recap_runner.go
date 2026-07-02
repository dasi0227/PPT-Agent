package assist

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

// RecapRunner 实现 /recap：只读汇总项目状态（主题/页数/各页标题版式/最近变更），
// 结构化输出到 info 事件，零产物（SPEC-CMD-RECAP-001/002/004）。满足 run.Runner。
type RecapRunner struct {
	store     Store
	runID     string
	projectID string
}

func NewRecapRunner(store Store, runID, projectID string) *RecapRunner {
	return &RecapRunner{store: store, runID: runID, projectID: projectID}
}

func (r *RecapRunner) Run(ctx context.Context, em harness.Emitter, _ harness.Checkpointer, _ run.Prompter) harness.Outcome {
	em.Emit(model.EventRunStarted, harness.RunStartedPayload{
		RunID: r.runID, Kind: string(model.KindCommand), Scope: string(model.ScopeCurrent), Mode: string(model.ModeNormal),
	})

	proj, err := r.store.GetProject(ctx, r.projectID)
	if err != nil {
		em.Emit(model.EventError, harness.ErrorPayload{Code: "BAD_STATE", Message: err.Error()})
		return harness.Outcome{Status: harness.OutcomeLLMError, Code: "BAD_STATE", Message: err.Error()}
	}
	slides, err := r.store.ListSlides(ctx, r.projectID)
	if err != nil {
		em.Emit(model.EventError, harness.ErrorPayload{Code: "INTERNAL", Message: err.Error()})
		return harness.Outcome{Status: harness.OutcomeLLMError, Code: "INTERNAL", Message: err.Error()}
	}

	summary := r.render(ctx, proj, slides)
	em.Emit(model.EventInfo, harness.InfoPayload{Text: summary})
	return harness.Outcome{Status: harness.OutcomeFinished, Summary: "recap 完成"}
}

// render 生成结构化摘要（列表形式，避免散文；SPEC-CMD-RECAP-004）。
func (r *RecapRunner) render(ctx context.Context, proj model.Project, slides []model.Slide) string {
	var b strings.Builder
	fmt.Fprintf(&b, "项目：%s | 主题：%s | %d 页 | 状态：%s\n", proj.Title, proj.Theme, len(slides), proj.Status)

	b.WriteString("页面：\n")
	for _, s := range slides {
		fmt.Fprintf(&b, "  %d %-10s %s (v%d)\n", s.Idx, s.Layout, s.Title, s.CurrentVersion)
	}

	// 最近变更：聚合各页 slide 版本 + design 版本，按 CreatedAt 取最近若干条。
	recent := r.recentChanges(ctx, slides)
	b.WriteString("最近变更：\n")
	if len(recent) == 0 {
		b.WriteString("  （暂无版本记录）\n")
	}
	for _, line := range recent {
		b.WriteString("  - " + line + "\n")
	}
	return b.String()
}

func (r *RecapRunner) recentChanges(ctx context.Context, slides []model.Slide) []string {
	type change struct {
		at   int64
		text string
	}
	var all []change
	for _, s := range slides {
		vs, err := r.store.ListVersions(ctx, "slide", fmt.Sprintf("slide-%03d", s.Idx))
		if err != nil {
			continue
		}
		for _, v := range vs {
			all = append(all, change{at: v.CreatedAt, text: fmt.Sprintf("第%d页 v%d (run %s)", s.Idx, v.VersionNo, shortID(v.RunID))})
		}
	}
	for _, v := range must(r.store.ListVersions(ctx, "design", "design")) {
		all = append(all, change{at: v.CreatedAt, text: fmt.Sprintf("公共层 v%d (run %s)", v.VersionNo, shortID(v.RunID))})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].at > all[j].at })
	if len(all) > 5 {
		all = all[:5]
	}
	out := make([]string, len(all))
	for i, c := range all {
		out[i] = c.text
	}
	return out
}

func must(vs []model.Version, _ error) []model.Version { return vs }

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
