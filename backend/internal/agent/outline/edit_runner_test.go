package outline

import (
	"context"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/agent/slidejson"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm/llmtest"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// fakeEditor 是 OutlineEditor 的内存实现，供 edit runner 测试。
type fakeEditor struct {
	slides   []model.Slide
	content  map[string]slidejson.SlideJSON
	deleted  []string
	reorders [][]string
	adds     int
}

func newFakeEditor() *fakeEditor {
	return &fakeEditor{
		slides: []model.Slide{
			{ID: "a", ProjectID: "p1", Idx: 0, Order: 10, Layout: "cover", Title: "封面"},
			{ID: "b", ProjectID: "p1", Idx: 1, Order: 20, Layout: "bullets", Title: "背景"},
			{ID: "c", ProjectID: "p1", Idx: 2, Order: 30, Layout: "thanks", Title: "谢谢"},
		},
		content: map[string]slidejson.SlideJSON{
			"a": {ID: "a", Idx: 0, Layout: "cover", Title: "封面"},
			"b": {ID: "b", Idx: 1, Layout: "bullets", Title: "背景", Bullets: []string{"旧点"}},
			"c": {ID: "c", Idx: 2, Layout: "thanks", Title: "谢谢"},
		},
	}
}

func (f *fakeEditor) ListSlides(_ context.Context, _ string) ([]model.Slide, error) {
	return f.slides, nil
}
func (f *fakeEditor) PatchOutline(_ context.Context, slideID string, p OutlinePatch) (slidejson.SlideJSON, error) {
	cur := f.content[slideID]
	if p.Title != nil {
		cur.Title = *p.Title
	}
	if p.Bullets != nil {
		cur.Bullets = *p.Bullets
	}
	f.content[slideID] = cur
	return cur, nil
}
func (f *fakeEditor) AddOutline(_ context.Context, _, _, layout string) (model.Slide, error) {
	f.adds++
	return model.Slide{ID: "new", ProjectID: "p1", Layout: layout, Order: 25}, nil
}
func (f *fakeEditor) DeleteOutline(_ context.Context, slideID string) error {
	f.deleted = append(f.deleted, slideID)
	return nil
}
func (f *fakeEditor) ReorderOutline(_ context.Context, _ string, orderedIDs []string) error {
	f.reorders = append(f.reorders, orderedIDs)
	return nil
}

// fakePrompter 记录 NeedsInput 调用并返回预置应答。
type fakePrompter struct {
	reply  string
	called int
}

func (p *fakePrompter) NeedsInput(_ context.Context, _, _ string, _ []string) (string, error) {
	p.called++
	return p.reply, nil
}

func newEditRunner(t *testing.T, editor OutlineEditor, client llm.Client) *EditRunner {
	t.Helper()
	return NewEditRunner(client, editor, EditParams{RunID: "r1", ProjectID: "p1", Language: "zh"})
}

// TestEditRunnerPatch：patch_outline_slide 改标题 → editor content 更新。
func TestEditRunnerPatch(t *testing.T) {
	editor := newFakeEditor()
	fake := &llmtest.FakeClient{Script: []llm.ToolCallResponse{
		{ToolCall: &llm.ToolCall{ID: "c1", Name: "patch_outline_slide", Args: map[string]any{"slide_id": "b", "title": "新背景"}}},
		{ToolCall: &llm.ToolCall{ID: "c2", Name: "finish", Args: map[string]any{"summary": "done"}}},
	}}
	r := newEditRunner(t, editor, fake)
	out := r.Run(context.Background(), &captureEmitter{}, nil, nil)
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("want finished, got %s (%s)", out.Status, out.Message)
	}
	if editor.content["b"].Title != "新背景" {
		t.Fatalf("patch not applied: %+v", editor.content["b"])
	}
}

// TestEditRunnerDeleteConfirmed：delete_outline_slide → 发 needs_input → 回答"确认删除" → 页被删。
func TestEditRunnerDeleteConfirmed(t *testing.T) {
	editor := newFakeEditor()
	fake := &llmtest.FakeClient{Script: []llm.ToolCallResponse{
		{ToolCall: &llm.ToolCall{ID: "c1", Name: "delete_outline_slide", Args: map[string]any{"slide_id": "b"}}},
		{ToolCall: &llm.ToolCall{ID: "c2", Name: "finish", Args: map[string]any{"summary": "done"}}},
	}}
	prompter := &fakePrompter{reply: "确认删除"}
	r := newEditRunner(t, editor, fake)
	out := r.Run(context.Background(), &captureEmitter{}, nil, prompter)
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("want finished, got %s", out.Status)
	}
	if prompter.called != 1 {
		t.Fatalf("expected NeedsInput called once, got %d", prompter.called)
	}
	if len(editor.deleted) != 1 || editor.deleted[0] != "b" {
		t.Fatalf("expected slide b deleted after confirm, got %v", editor.deleted)
	}
}

// TestEditRunnerDeleteCanceled：回答不含"确认" → 不删页。
func TestEditRunnerDeleteCanceled(t *testing.T) {
	editor := newFakeEditor()
	fake := &llmtest.FakeClient{Script: []llm.ToolCallResponse{
		{ToolCall: &llm.ToolCall{ID: "c1", Name: "delete_outline_slide", Args: map[string]any{"slide_id": "b"}}},
		{ToolCall: &llm.ToolCall{ID: "c2", Name: "finish", Args: map[string]any{"summary": "done"}}},
	}}
	prompter := &fakePrompter{reply: "取消"}
	r := newEditRunner(t, editor, fake)
	out := r.Run(context.Background(), &captureEmitter{}, nil, prompter)
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("want finished, got %s", out.Status)
	}
	if prompter.called != 1 {
		t.Fatalf("expected NeedsInput called once, got %d", prompter.called)
	}
	if len(editor.deleted) != 0 {
		t.Fatalf("expected no delete on cancel, got %v", editor.deleted)
	}
}

// TestEditRunnerReorder：reorder_outline_slides → editor 收到新顺序。
func TestEditRunnerReorder(t *testing.T) {
	editor := newFakeEditor()
	fake := &llmtest.FakeClient{Script: []llm.ToolCallResponse{
		{ToolCall: &llm.ToolCall{ID: "c1", Name: "reorder_outline_slides", Args: map[string]any{"ordered_ids": []any{"c", "a", "b"}}}},
		{ToolCall: &llm.ToolCall{ID: "c2", Name: "finish", Args: map[string]any{"summary": "done"}}},
	}}
	r := newEditRunner(t, editor, fake)
	out := r.Run(context.Background(), &captureEmitter{}, nil, nil)
	if out.Status != harness.OutcomeFinished {
		t.Fatalf("want finished, got %s", out.Status)
	}
	if len(editor.reorders) != 1 || strings.Join(editor.reorders[0], ",") != "c,a,b" {
		t.Fatalf("reorder not applied: %v", editor.reorders)
	}
}
