// plan.go 是 v2 生成流水线的规划节点（V2-AGENT-PIPELINE §4）。
// 确定性地从大纲 slide-json[] 生成逐页构建计划，保证 plan 事件必发（LLM 无关）。
package generate

import (
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

const (
	planStatusPending    = "pending"
	planStatusInProgress = "in_progress"
	planStatusCompleted  = "completed"
	planStatusFailed     = "failed"

	planStepValidate = "validate"
	planStepDeliver  = "deliver"
)

// pageStepID 返回某页的 step id（与 plan.update 保持一致的命名，V2-SSE-002）。
func pageStepID(idx int) string {
	return fmt.Sprintf("p%d", idx)
}

// buildPlan 从大纲确定性生成一份逐页构建计划：每页一条 step + 全局校验步 + 交付步。
// steps 初始状态均为 pending；后续由 runner 用 plan.update 增量推进。
func buildPlan(runID string, slides []model.Slide) harness.PlanPayload {
	steps := make([]harness.PlanStepPayload, 0, len(slides)+2)
	for _, sl := range slides {
		steps = append(steps, harness.PlanStepPayload{
			ID:     pageStepID(sl.Idx),
			Title:  fmt.Sprintf("第 %d 页 · %s（%s）", sl.Idx+1, pageTitle(sl), sl.Layout),
			Status: planStatusPending,
			Detail: fmt.Sprintf("layout=%s", sl.Layout),
		})
	}
	steps = append(steps,
		harness.PlanStepPayload{ID: planStepValidate, Title: "全局校验", Status: planStatusPending},
		harness.PlanStepPayload{ID: planStepDeliver, Title: "交付", Status: planStatusPending},
	)
	return harness.PlanPayload{
		ID:    "plan_" + runID,
		Title: fmt.Sprintf("构建 %d 页", len(slides)),
		Steps: steps,
	}
}

func pageTitle(sl model.Slide) string {
	if sl.Title != "" {
		return sl.Title
	}
	return fmt.Sprintf("第 %d 页", sl.Idx+1)
}

// planStep 是 plan 的一个 step 的轻量视图（供 runner 取标题/详情）。
type planStep struct {
	Title  string
	Detail string
}

// stepByID 返回指定 step 的标题/详情；未命中返回空。
func stepByID(p harness.PlanPayload, id string) planStep {
	for _, s := range p.Steps {
		if s.ID == id {
			return planStep{Title: s.Title, Detail: s.Detail}
		}
	}
	return planStep{}
}
