package outline

import (
	"context"
	"strconv"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/run"
)

// EditParams 是一次大纲编辑 Run 的输入。
type EditParams struct {
	RunID       string
	ProjectID   string
	Instruction string
	Language    string
	Mode        model.Mode // normal / ask
	ContextPack *contextengine.ContextPack
}

// EditRunner 用 harness ReAct 循环执行大纲编辑（patch/add/delete/reorder），满足 run.Runner。
// 与手动 REST 共用同一底层 service（经 OutlineEditor 适配），删页走 needs_input 二次确认。
type EditRunner struct {
	client llm.Client
	editor OutlineEditor
	params EditParams
}

// NewEditRunner 构造大纲编辑 runner。
func NewEditRunner(client llm.Client, editor OutlineEditor, p EditParams) *EditRunner {
	return &EditRunner{client: client, editor: editor, params: p}
}

func (r *EditRunner) Run(ctx context.Context, em harness.Emitter, cp harness.Checkpointer, prompter run.Prompter) harness.Outcome {
	// 快照当前大纲，注入 prompt 供 LLM 定位页 id。
	var slides []model.Slide
	if r.params.ContextPack != nil {
		slides = make([]model.Slide, 0, len(r.params.ContextPack.Deck.Summaries))
		for idx, summary := range r.params.ContextPack.Deck.Summaries {
			slides = append(slides, model.Slide{ID: summary.ID, Idx: idx, Layout: summary.Role, Title: summary.Title})
		}
	} else {
		var err error
		slides, err = r.editor.ListSlides(ctx, r.params.ProjectID)
		if err != nil {
			em.Emit(model.EventError, harness.ErrorPayload{Code: "INTERNAL", Message: err.Error()})
			return harness.Outcome{Status: harness.OutcomeLLMError, Code: "INTERNAL", Message: err.Error()}
		}
	}

	toolset := []tools.Tool{
		&patchOutlineTool{editor: r.editor},
		&addOutlineTool{editor: r.editor, projectID: r.params.ProjectID},
		&deleteOutlineTool{editor: r.editor, prompter: prompter},
		&reorderOutlineTool{editor: r.editor, projectID: r.params.ProjectID},
		tools.NewFinishTool(),
	}
	if refTool := contextengine.RefTool(r.params.ContextPack); refTool != nil {
		toolset = append(toolset, refTool)
	}

	mode := r.params.Mode
	if mode == "" {
		mode = model.ModeNormal
	}
	pp := prompt.OutlineEditParams{
		Instruction: r.params.Instruction,
		Language:    r.params.Language,
		Layouts:     slidejson.LayoutEnum(),
		Slides:      outlineDigest(slides),
	}

	systemPrompt, userPrompt := contextengine.CompileForRunner(r.params.ContextPack, prompt.OutlineEditSystem(pp), prompt.OutlineEditUser(pp))
	loop := harness.New(r.client, harness.Config{
		RunID:        r.params.RunID,
		Kind:         model.KindOutline,
		Scope:        model.ScopeCurrent,
		Mode:         mode,
		SystemPrompt: systemPrompt,
		Instruction:  userPrompt,
		Tools:        toolset,
	})
	return loop.Run(ctx, em, cp)
}

// outlineDigest 把当前大纲压成"id | idx | layout | title"多行摘要，供 prompt 注入。
func outlineDigest(slides []model.Slide) string {
	var b strings.Builder
	for _, s := range slides {
		b.WriteString(s.ID)
		b.WriteString(" | idx=")
		b.WriteString(strconv.Itoa(s.Idx))
		b.WriteString(" | ")
		b.WriteString(s.Layout)
		b.WriteString(" | ")
		b.WriteString(s.Title)
		b.WriteString("\n")
	}
	return b.String()
}
