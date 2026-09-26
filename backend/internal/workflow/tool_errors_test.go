package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
)

func errorObservation(t *testing.T, result ToolResult) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(result.Observation), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestToolFailureDetailsSurviveBatchAndReplay(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	registry := NewToolRegistry()
	if err := (DefaultDomainToolProvider{Pack: pack}).RegisterDomainTools(registry); err != nil {
		t.Fatal(err)
	}
	call := llm.ToolCall{ID: "bad-array", Name: "edit_design", Args: map[string]any{"layout_preferences": "not JSON"}}
	results := NewRuntime(nil).executeToolBatch(context.Background(), RuntimeInput{RunID: "errors", ProjectDir: dir}, batchState(pack), registry, map[string]bool{"edit_design": true}, []llm.ToolCall{call})
	result := results[0]
	raw, err := marshalPersistedToolResult(result)
	if err != nil {
		t.Fatal(err)
	}
	var persisted persistedToolResult
	if err := json.Unmarshal([]byte(raw), &persisted); err != nil {
		t.Fatal(err)
	}
	for _, got := range []ToolResult{result, bindToolErrorObservation(persisted.Result, call)} {
		view := errorObservation(t, got)
		if view["field"] != "/layout_preferences" || view["expected"] != "array" || view["actual"] != "string" || view["call_id"] != call.ID || view["category"] != "agent_repairable" {
			t.Fatalf("repair details lost: %v", view)
		}
	}
	command := commandFailure(commandexec.Decision{}, commandexec.Result{ExitCode: 2, Stderr: "invalid regular expression", Stdout: "partial"}, &commandexec.Error{Code: commandexec.CodeExitNonzero, Message: "command exited with status 2"})
	view := errorObservation(t, bindToolErrorObservation(command, llm.ToolCall{ID: "cmd", Name: "run_command"}))
	if view["stderr"] != "invalid regular expression" || view["exit_code"] != float64(2) || view["category"] != "agent_repairable" {
		t.Fatalf("command diagnostics lost: %v", view)
	}
}

func TestOutlineLogFailuresGiveActionableFeedbackAndLeaveNoPartialFile(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	path := filepath.Join(dir, ".outline.json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	session, err := NewRunSession(dir, "outline-errors")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Discard()
	input := DomainToolInput{ProjectDir: dir, Session: session, Context: pack, Scope: pack.Command.Scope, Args: map[string]any{"resource": "outline"}}
	read := (pptReadTool{pack: pack}).Execute(context.Background(), input)
	if read.Code != "OUTLINE_NOT_INITIALIZED" || !strings.Contains(errorObservation(t, read)["next_action"].(string), "init_outline") {
		t.Fatalf("missing outline feedback: %+v", read)
	}
	tool := resourceEditTool{pack: pack, name: "init_outline"}
	for _, page := range []string{`{"id":"sli_cover","title":"Opening","purpose":"Explain"}`, `{"id":"sli_cover","title":"Opening"}`} {
		input.Args = map[string]any{"content": `{"sections":[{"title":"Intro","purpose":"Explain","slides":[` + page + `],"subsections":[]}]}`}
		result := bindToolErrorObservation(tool.Execute(context.Background(), input), llm.ToolCall{ID: "init", Name: "init_outline", Args: input.Args})
		view := errorObservation(t, result)
		if result.Code != CodeContentInvalid || view["field"] != "/sections/0/slides/0" || !strings.Contains(result.Observation, "additionalProperties") || !strings.Contains(view["next_action"].(string), "only title") {
			t.Fatalf("outline error lost corrective detail: %v", view)
		}
		if _, err := session.ReadPath(".outline.json"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("failed initialization left content: %v", err)
		}
	}
	input.Args = map[string]any{"content": `{"sections":[{"title":"Intro","purpose":"Explain","slides":[{"title":"Opening"}],"subsections":[]}]}`}
	if result := tool.Execute(context.Background(), input); !result.OK {
		t.Fatalf("corrected initialization failed: %+v", result)
	}
	if result := tool.Execute(context.Background(), input); result.Code != "TARGET_ALREADY_EXISTS" {
		t.Fatalf("existing outline was not protected: %+v", result)
	}
}

func TestResourceFailuresDistinguishMissingCorruptAndTextMatches(t *testing.T) {
	for _, part := range []string{"outline", "html"} {
		resource := Resource{Type: "deck", Part: part}
		for _, text := range []string{"unmatched", "same same"} {
			_, err := pptmutation.ApplyTextEdits([]byte(text), []pptmutation.Edit{{OldText: "same", NewText: "new"}})
			result := resourceMutationFailure(err, resource)
			view := errorObservation(t, result)
			want := "EDIT_ANCHOR_NOT_FOUND"
			if text == "same same" {
				want = "EDIT_ANCHOR_AMBIGUOUS"
			}
			if result.Code != want || view["field"] != "/edits/0/old_text" || view["match_count"] == nil || !strings.Contains(view["next_action"].(string), "read_resource") {
				t.Fatalf("text edit diagnostics: %+v", view)
			}
		}
	}
	dir, _, pack := generationPackFixture(t)
	path := filepath.Join(dir, ".outline.json")
	if err := os.WriteFile(path, []byte(`{"sections":`), 0o600); err != nil {
		t.Fatal(err)
	}
	read := (pptReadTool{pack: pack}).Execute(context.Background(), DomainToolInput{ProjectDir: dir, Args: map[string]any{"resource": "outline"}})
	if read.Code != "RESOURCE_CONTENT_INVALID" || strings.Contains(errorObservation(t, read)["next_action"].(string), "use init_outline") {
		t.Fatalf("corruption treated as absence: %+v", read)
	}
	for _, part := range []string{"spec", "html"} {
		result := resourceReadFailure(fs.ErrNotExist, Resource{Type: "slide", Part: part})
		want := "edit_spec"
		if part == "html" {
			want = "write_html"
		}
		if !strings.Contains(errorObservation(t, result)["next_action"].(string), want) {
			t.Fatalf("missing %s feedback: %+v", part, result)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, model.SpecCollectionPath), []byte(`{"invalid_slide_key":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	read = (pptReadTool{pack: pack}).Execute(context.Background(), DomainToolInput{ProjectDir: dir, Args: map[string]any{"resource": "spec", "slide_id": "sli_example"}})
	if read.Code != "RESOURCE_CONTENT_INVALID" {
		t.Fatalf("damaged spec collection treated as absent: %+v", read)
	}
	for _, code := range []string{"SKILL_DISABLED", "SKILL_NOT_FOUND", CodeContextBudget} {
		result := repositoryLoadFailure("skill", code, "cannot load requested skills")
		if model.ErrorDefinitionFor(result.Code).Category != model.ErrorAgentRepairable || !strings.Contains(errorObservation(t, result)["next_action"].(string), "skill") {
			t.Fatalf("repository load repair feedback: %+v", result)
		}
	}
}

func TestToolErrorCategoriesAndFlatResourceContract(t *testing.T) {
	if err := classifyProviderError(context.Background(), llm.ErrBadToolCall); err.Code != "MODEL_TOOL_CALL_INVALID" || err.ShouldAutoRetry() {
		t.Fatalf("undecodable protocol response was not distinguished from tool validation: %+v", err)
	}
	for _, code := range []string{
		commandexec.CodeParseInvalid, commandexec.CodeSyntaxDenied, commandexec.CodeNotAllowed,
		commandexec.CodeNotAvailable, commandexec.CodeFlagDenied, commandexec.CodePathOutsideProject,
		commandexec.CodePathInvalid, commandexec.CodeSensitiveDenied, commandexec.CodeExitNonzero,
		commandexec.CodeTimeout, commandexec.CodeOutputLimit, commandexec.CodeExecFailed,
		"COMMAND_PERMISSION_DENIED", "OUTLINE_NOT_INITIALIZED", "PLAN_INVALID", CodeToolArgumentInvalid,
	} {
		def := model.ErrorDefinitionFor(code)
		if def.Code != code || def.Category != model.ErrorAgentRepairable || def.Retryable {
			t.Fatalf("wrong repair category: %+v", def)
		}
	}
	for _, code := range []string{"READ_FAILED", "WRITE_FAILED", commandexec.CodeInvariantViolation} {
		if def := model.ErrorDefinitionFor(code); def.Code != code || def.Category != model.ErrorTerminal {
			t.Fatalf("infrastructure failure misclassified: %+v", def)
		}
	}
	for _, args := range []map[string]any{{"resource": "spec"}, {"resource": "outline", "slide_id": "sli_example"}, {"resource": map[string]any{"kind": "outline"}}} {
		if _, _, err := prepareToolArguments((pptReadTool{}).Schema(), args); err == nil {
			t.Fatalf("accepted invalid resource parameters: %v", args)
		}
	}
}
