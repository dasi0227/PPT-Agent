package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func renderThemeFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "pro_aaaaaa", "artifacts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "seed", "assets", "themes", "editorial-serif", "theme.css"))
	if err != nil {
		t.Fatal(err)
	}
	themePath := filepath.Join(root, "assets", "themes", "clean", "theme.css")
	if err := os.MkdirAll(filepath.Dir(themePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(themePath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	return dir, string(raw)
}

type resourceApprovalFixture struct {
	mu   sync.Mutex
	rows map[string]model.ResourceEditApproval
	next CheckpointSink
}

func withResourceApprovals(input RuntimeInput) RuntimeInput {
	f := &resourceApprovalFixture{rows: map[string]model.ResourceEditApproval{}, next: input.Checkpoint}
	input.Checkpoint, input.ResourceApproval = f, f.read
	input.Prompter = autoApprovingResourcePrompter{approvals: f}
	return input
}

func (f *resourceApprovalFixture) SaveCheckpoint(ctx context.Context, cp RuntimeCheckpoint) error {
	if pending := cp.PendingResourceApproval; pending != nil && pending.Candidate != nil {
		f.mu.Lock()
		if _, exists := f.rows[pending.InteractionID]; !exists {
			f.rows[pending.InteractionID] = model.ResourceEditApproval{ResourceEditCandidate: *pending.Candidate,
				RunID: cp.RunID, CallID: pending.CallID, InteractionID: pending.InteractionID,
				Resource: pending.Request.Resource, Revision: pending.Request.Revision, Target: pending.Request.Target, State: "pending"}
		}
		f.mu.Unlock()
	}
	if f.next != nil {
		return f.next.SaveCheckpoint(ctx, cp)
	}
	return nil
}

func (f *resourceApprovalFixture) read(_ context.Context, runID, id string) (model.ResourceEditApproval, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	record, ok := f.rows[id]
	if !ok || record.RunID != runID {
		return model.ResourceEditApproval{}, errors.New("approval not saved with checkpoint")
	}
	return record, nil
}

func (f *resourceApprovalFixture) accept(answer model.ResourceEditApprovalAnswer) (model.ResourceEditApprovalAnswer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	record, ok := f.rows[answer.InteractionID]
	if !ok || record.CallID != answer.CallID || record.Revision != answer.Revision {
		return answer, ErrResourceApprovalConflict
	}
	answer.Feedback = strings.TrimSpace(answer.Feedback)
	record.State, record.Answer = "answered", &answer
	f.rows[answer.InteractionID] = record
	return answer, nil
}

type waitingResourcePrompter struct {
	approvals *resourceApprovalFixture
	requested chan model.ResourceEditApprovalRequestedPayload
	answered  chan model.ResourceEditApprovalAnswer
}

func (waitingResourcePrompter) Ask(context.Context, model.QuestionAskedPayload) (model.QuestionAnswer, error) {
	return model.QuestionAnswer{}, nil
}

func (p waitingResourcePrompter) AskResourceEditApproval(ctx context.Context, payload model.ResourceEditApprovalRequestedPayload) (model.ResourceEditApprovalAnswer, error) {
	p.requested <- payload
	select {
	case answer := <-p.answered:
		return p.approvals.accept(answer)
	case <-ctx.Done():
		return model.ResourceEditApprovalAnswer{}, ctx.Err()
	}
}

func testResourceMessages(t *testing.T, dir string, pack contextengine.ContextPack) []llm.Message {
	t.Helper()
	messages := []llm.Message{}
	for _, resource := range []Resource{{Type: "deck", Part: "meta"}, {Type: "deck", Part: "constraints"}, {Type: "deck", Part: "outline"}, {Type: "slide", Part: "spec", SlideID: generationSlide}, {Type: "slide", Part: "html", SlideID: generationSlide}} {
		args := map[string]any{"type": resource.Part}
		if resource.SlideID != "" {
			args["slide_id"] = resource.SlideID
		}
		result := (pptReadTool{}).Execute(context.Background(), DomainToolInput{Args: args, Context: pack, ProjectDir: dir, Scope: pack.Command.Scope})
		if result.OK {
			messages = appendBatchObservations(messages, []llm.ToolCall{{ID: resource.Key(), Name: "read_resource", Args: args}}, "", []ToolResult{result})
		}
	}
	return messages
}

func assertSavedResourceObservation(t *testing.T, tool string, result ToolResult, saved []byte) {
	t.Helper()
	var content any
	if tool == "edit_outline" {
		content = string(saved)
	} else {
		if err := json.Unmarshal(saved, &content); err != nil {
			t.Fatal(err)
		}
	}
	want := map[string]any{"content": content}
	if !result.OK || !result.Applied || !reflect.DeepEqual(result.Data, want) {
		t.Fatalf("%s result differs from saved content: %+v", tool, result)
	}
	want["status"] = "success"
	raw, err := marshalPersistedToolResult(result, nil)
	if err != nil {
		t.Fatal(err)
	}
	var receipt persistedToolResult
	if err := json.Unmarshal([]byte(raw), &receipt); err != nil {
		t.Fatal(err)
	}
	call := llm.ToolCall{ID: "saved-resource", Name: tool}
	single := appendToolObservation(nil, call, "", result)
	batch := appendBatchObservations(nil, []llm.ToolCall{call}, "", []ToolResult{result})
	for _, observation := range []string{single[len(single)-1].Text(), batch[len(batch)-1].Text(), modelToolObservation(receipt.Result)} {
		var value map[string]any
		if err := json.Unmarshal([]byte(observation), &value); err != nil || !reflect.DeepEqual(value, want) {
			t.Fatalf("%s observation differs from saved content: %s (%v)", tool, observation, err)
		}
	}
}

func objectSchema(required []string, properties map[string]any) map[string]any {
	schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

const generationSlide = "sli_aaaaaa"

const generationHTML = `<!doctype html><html><body><section class="slide-stage"><h1>Original</h1></section></body></html>`

func generationPackFixture(t *testing.T) (string, string, contextengine.ContextPack) {
	t.Helper()
	dir, css := renderThemeFixture(t)
	manifest := spec.Meta{Title: "Deck", Goal: "Explain", Audience: "Builders", Language: "zh-CN", Count: "待明确"}
	design := spec.Constraints{Content: spec.ConstraintSet{Requirements: []string{}, Prohibitions: []string{}}, Visual: spec.ConstraintSet{Requirements: []string{"A"}, Prohibitions: []string{}}}
	slide := spec.SlideSpec{Purpose: "content_explanation", Layout: "Title above content", Core: "Message", Elements: []spec.Element{}}
	pack := testPack(model.ModeCreate, model.ScopeCurrentPage, false, "Update requirements")
	pack.Project.ThemeID = "clean"
	pack.Command.Scope = model.NewRunScope(model.ScopeCurrentPage, generationSlide)
	pack.PresentationMeta.Meta = manifest
	pack.Constraints.Constraints = &design
	pack.Outline.Outline = spec.Outline{Sections: []spec.Section{{ID: "sec_aaaaaa", Title: "Section", Slides: []spec.SlideNode{{ID: generationSlide, Title: "Page"}}, Subsections: []spec.Subsection{}}}}
	pack.Outline.Summaries = []contextengine.SlideSummary{{ID: generationSlide, State: string(model.HTMLAvailable)}}
	for path, content := range map[string]any{".meta.json": manifest, ".outline.json": pack.Outline.Outline, ".constraints.json": design, model.SlideSpecPath(generationSlide): slide} {
		raw, _ := json.Marshal(content)
		writeGenerationFile(t, dir, path, raw)
	}
	writeGenerationFile(t, dir, model.SlideHTMLPath(generationSlide), []byte(generationHTML))
	return dir, css, pack
}

func writeGenerationFile(t *testing.T, dir, path string, raw []byte) {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, raw, 0644); err != nil {
		t.Fatal(err)
	}
}

type autoApprovingResourcePrompter struct{ approvals *resourceApprovalFixture }

func (autoApprovingResourcePrompter) Ask(context.Context, model.QuestionAskedPayload) (model.QuestionAnswer, error) {
	return model.QuestionAnswer{}, nil
}

func (p autoApprovingResourcePrompter) AskResourceEditApproval(_ context.Context, payload model.ResourceEditApprovalRequestedPayload) (model.ResourceEditApprovalAnswer, error) {
	answer := model.ResourceEditApprovalAnswer{InteractionID: payload.InteractionID, CallID: payload.CallID, Revision: payload.Revision, Decision: "approve"}
	return p.approvals.accept(answer)
}

const otherBatchSlide = "sli_bbbbbb"

type gatedResourceTool struct {
	DomainTool
	before func(context.Context, DomainToolInput) error
}

func (t gatedResourceTool) Execute(ctx context.Context, input DomainToolInput) ToolResult {
	if t.before != nil {
		if err := t.before(ctx, input); err != nil {
			return internalToolFailure(err)
		}
	}
	return t.DomainTool.Execute(ctx, input)
}

func authoringBatchFixture(t *testing.T, hook func(context.Context, DomainToolInput) error) (RuntimeInput, *RunState, *ToolRegistry) {
	t.Helper()
	dir, _, pack := generationPackFixture(t)
	pack.Command.Scope = model.NewRunScope(model.ScopeAllPages, generationSlide, otherBatchSlide)
	pack.Outline.Outline.Sections[0].Slides = append(pack.Outline.Outline.Sections[0].Slides, spec.SlideNode{ID: otherBatchSlide, Title: "Other"})
	raw, _ := json.Marshal(pack.Outline.Outline)
	writeGenerationFile(t, dir, ".outline.json", raw)
	raw, _ = json.Marshal(spec.SlideSpec{Purpose: "content_explanation", Layout: "Title above content", Core: "Message", Elements: []spec.Element{}})
	for _, id := range []string{generationSlide, otherBatchSlide} {
		writeGenerationFile(t, dir, model.SlideSpecPath(id), raw)
	}
	writeGenerationFile(t, dir, model.SlideHTMLPath(otherBatchSlide), []byte(generationHTML))
	state := batchState(pack)
	state.tx, _ = NewRunSession(dir, state.runID)
	t.Cleanup(state.tx.Discard)
	state.messages = testResourceMessages(t, dir, pack)
	for _, part := range []string{"spec", "html"} {
		args := map[string]any{"type": part, "slide_id": otherBatchSlide}
		result := (pptReadTool{}).Execute(context.Background(), DomainToolInput{Args: args, Context: pack, ProjectDir: dir, Scope: pack.Command.Scope})
		state.messages = appendBatchObservations(state.messages, []llm.ToolCall{{ID: "read_" + part, Name: "read_resource", Args: args}}, "", []ToolResult{result})
	}
	state.rememberResourceVersions()
	registry := NewToolRegistry()
	for _, name := range []string{"edit_meta", "edit_constraints", "edit_outline", "edit_spec", "edit_html"} {
		if err := registry.Register(gatedResourceTool{DomainTool: resourceEditTool{name: name}, before: hook}, false, CapabilityPPTMutate, RiskMedium, PhaseExecuting); err != nil {
			t.Fatal(err)
		}
	}
	if err := registry.Register(pptReadTool{}, true, CapabilityRead, RiskLow, PhaseExecuting); err != nil {
		t.Fatal(err)
	}
	return withResourceApprovals(RuntimeInput{RunID: state.runID, ProjectDir: dir, Context: pack}), state, registry
}

func batchState(pack contextengine.ContextPack) *RunState {
	return &RunState{
		runID: "batch", loopID: "loop-batch", phase: PhaseExecuting,
		scope: pack.Command.Scope, mode: pack.Command.Mode, pack: pack, ledger: NewEvidenceLedger(),
	}
}
