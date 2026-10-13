package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
)

func errorObservation(t *testing.T, result ToolResult) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(result.Observation), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestEditingErrorContractSurvivesDisclosureAndReplay(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(resourceEditTool{name: "edit_html"}, false, CapabilityWrite, RiskMedium, PhaseExecuting); err != nil {
		t.Fatal(err)
	}
	tools := []llm.ToolSchema{}
	for _, tool := range registry.Disclose(PhaseExecuting, model.ModeExecute, model.NewRunScope(model.ScopeAllPages, generationSlide)) {
		tools = append(tools, llm.ToolSchema{Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters})
	}
	if len(tools) != 1 || !strings.Contains(tools[0].Description, "Returns:") || !strings.Contains(tools[0].Description, "Errors:") || !strings.Contains(tools[0].Description, "CONTENT_CONFLICT") {
		t.Fatalf("model documentation missing: %v", tools)
	}
	resource := Resource{Type: "slide", Part: "html", SlideID: generationSlide}
	call := llm.ToolCall{ID: "dependent", Name: "edit_html", Args: map[string]any{"slide_id": generationSlide}}
	_, anchorErr := pptmutation.ApplyTextEdits([]byte("same same"), []pptmutation.Edit{{OldText: "same", NewText: "new"}})
	for _, failure := range []ToolResult{
		resourceConflictFailure(resource, true), resourceConflictFailure(resource, false),
		resourceMutationFailure(anchorErr, resource), resourceMutationFailure(pptmutation.ErrSlideNotFound, resource),
		dependencyFailure([]string{"prerequisite", "other-prerequisite"}),
	} {
		original := bindToolErrorObservation(failure, call)
		raw, err := marshalPersistedToolResult(original, nil)
		if err != nil {
			t.Fatal(err)
		}
		var saved persistedToolResult
		if err := json.Unmarshal([]byte(raw), &saved); err != nil {
			t.Fatal(err)
		}
		for _, result := range []ToolResult{original, bindToolErrorObservation(saved.Result, call)} {
			messages := appendBatchObservations(nil, []llm.ToolCall{call}, "", []ToolResult{result})
			var observation map[string]any
			if err := json.Unmarshal([]byte(messages[len(messages)-1].Text()), &observation); err != nil {
				t.Fatal(err)
			}
			if messages[len(messages)-1].ToolCallID != call.ID {
				t.Fatalf("outer call association lost: %v", messages)
			}
			for _, key := range []string{"code", "category", "reason", "measure"} {
				if value, ok := observation[key].(string); !ok || value == "" {
					t.Fatalf("missing required error field %s: %v", key, observation)
				}
			}

			if result.Code == CodeContentConflict && (!strings.Contains(observation["measure"].(string), generationSlide) || !strings.Contains(observation["measure"].(string), `"type":"html"`)) {
				t.Fatalf("conflict omitted actual read parameters: %v", observation)
			}
			if result.Code == CodeDependencyFailed && !reflect.DeepEqual(observation["dependency_call_ids"], []any{"prerequisite", "other-prerequisite"}) {
				t.Fatalf("failed prerequisite lost: %v", observation)
			}
		}
	}
	t.Run("render error retains its screenshot through replay", func(t *testing.T) {
		image := renderimage.Entry{ProjectID: "p1", RunID: "render", ScreenshotID: "shot", SlideID: generationSlide, SourceHash: "source"}
		rendered := slideRenderResult(image, RenderProof{}, RenderDiagnostics{
			Overflow: map[string]bool{"horizontal": true}, ContentSize: map[string]int{"width": 2000, "height": 1080},
		}, "test", false)
		call := llm.ToolCall{ID: "render", Name: "render_slide", Args: map[string]any{"slide_id": generationSlide}}
		original := bindToolErrorObservation(rendered, call)
		raw, err := marshalPersistedToolResult(original, nil)
		if err != nil {
			t.Fatal(err)
		}
		var saved persistedToolResult
		if err := json.Unmarshal([]byte(raw), &saved); err != nil {
			t.Fatal(err)
		}
		saved.Result.ObservationParts = saved.ObservationParts
		for _, result := range []ToolResult{original, bindToolErrorObservation(saved.Result, call)} {
			parts := result.ObservationParts
			if result.OK || len(parts) != 2 || parts[1].Type != "image" || parts[1].ImageRef != image.ImageRef() {
				t.Fatalf("render failure lost its image: %+v", result)
			}
			view := errorObservation(t, result)
			if view["code"] != CodeRenderFailed || view["category"] != "repairable" || view["reason"] == "" || view["measure"] == "" || view["diagnostics"] == nil || len(view) != 6 || parts[0].Text != result.Observation {
				t.Fatalf("render error protocol or diagnostics lost: %v", view)
			}
		}
	})
}

func TestToolFailureDetailsSurviveBatchAndReplay(t *testing.T) {
	dir, _, pack := generationPackFixture(t)
	registry := NewToolRegistry()
	if err := (DefaultDomainToolProvider{Pack: pack}).RegisterDomainTools(registry); err != nil {
		t.Fatal(err)
	}
	call := llm.ToolCall{ID: "bad-array", Name: "edit_design", Args: map[string]any{"requirements": "not JSON"}}
	results := NewRuntime(nil).executeToolBatch(context.Background(), RuntimeInput{RunID: "errors", ProjectDir: dir}, batchState(pack), registry, map[string]bool{"edit_design": true}, []llm.ToolCall{call})
	result := results[0]
	raw, err := marshalPersistedToolResult(result, nil)
	if err != nil {
		t.Fatal(err)
	}
	var persisted persistedToolResult
	if err := json.Unmarshal([]byte(raw), &persisted); err != nil {
		t.Fatal(err)
	}
	for _, got := range []ToolResult{result, bindToolErrorObservation(persisted.Result, call)} {
		view := errorObservation(t, got)
		if len(view) != 4 || !strings.Contains(view["reason"].(string), "/requirements") || !strings.Contains(view["reason"].(string), "expected array, but got string") || view["category"] != "repairable" {
			t.Fatalf("repair details lost: %v", view)
		}
	}
	command := commandFailure(commandexec.Decision{}, commandexec.Result{ExitCode: 2, Stderr: "invalid regular expression", Stdout: "partial"}, &commandexec.Error{Code: commandexec.CodeExitNonzero, Message: "command exited with status 2"})
	view := errorObservation(t, bindToolErrorObservation(command, llm.ToolCall{ID: "cmd", Name: "run_command"}))
	if view["stderr"] != "invalid regular expression" || view["exit_code"] != float64(2) || view["category"] != "repairable" {
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
	input := DomainToolInput{ProjectDir: dir, Session: session, Context: pack, Scope: pack.Command.Scope, Args: map[string]any{"type": "outline"}}
	read := (pptReadTool{pack: pack}).Execute(context.Background(), input)
	if read.OK || read.Applied || read.Code != CodeResourceNotFound || read.ObservationMetadata != nil || !strings.Contains(errorObservation(t, read)["measure"].(string), "edit_outline(init:") {
		t.Fatalf("missing outline feedback: %+v", read)
	}
	tool := resourceEditTool{pack: pack, name: "edit_outline"}
	for _, page := range []string{`{"id":"sli_cover","title":"Opening","purpose":"Explain"}`, `{"id":"sli_cover","title":"Opening"}`} {
		var initial map[string]any
		_ = json.Unmarshal([]byte(`{"sections":[{"title":"Intro","purpose":"Explain","slides":[`+page+`],"subsections":[]}]}`), &initial)
		input.Args = map[string]any{"init": initial}
		result := bindToolErrorObservation(tool.Execute(context.Background(), input), llm.ToolCall{ID: "init", Name: "edit_outline", Args: input.Args})
		view := errorObservation(t, result)
		if result.Code != CodeContentInvalid || !strings.Contains(view["reason"].(string), "/sections/0/slides/0/id") || !strings.Contains(result.Observation, "must be an existing sli identity") || !strings.Contains(view["measure"].(string), "only title") {
			t.Fatalf("outline error lost corrective detail: %v", view)
		}
		if _, err := session.ReadPath(".outline.json"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("failed initialization left content: %v", err)
		}
	}
	var initial map[string]any
	_ = json.Unmarshal([]byte(`{"sections":[{"title":"Intro","purpose":"Explain","slides":[{"title":"Opening"}],"subsections":[]}]}`), &initial)
	input.Args = map[string]any{"init": initial}
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
			if result.Code != want || !strings.Contains(view["reason"].(string), "/edits/0/old_text") || view["match_count"] == nil || !strings.Contains(view["measure"].(string), "read_resource") {
				t.Fatalf("text edit diagnostics: %+v", view)
			}
		}
	}
	dir, _, pack := generationPackFixture(t)
	path := filepath.Join(dir, ".outline.json")
	outlineSource, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"sections":`), 0o600); err != nil {
		t.Fatal(err)
	}
	read := (pptReadTool{pack: pack}).Execute(context.Background(), DomainToolInput{ProjectDir: dir, Args: map[string]any{"type": "outline"}})
	if read.Code != "RESOURCE_INVALID" || strings.Contains(errorObservation(t, read)["measure"].(string), "edit_outline(init:") {
		t.Fatalf("corruption treated as absence: %+v", read)
	}
	for _, part := range []string{"spec", "html"} {
		input := DomainToolInput{ProjectDir: dir, Context: pack, Args: map[string]any{"type": part, "slide_id": generationSlide}}
		if result := (pptReadTool{}).Execute(context.Background(), input); result.Code != "RESOURCE_INVALID" || !strings.Contains(result.Summary, "outline") {
			t.Fatalf("%s did not report its invalid outline dependency: %+v", part, result)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"outline", "spec", "html"} {
		args := map[string]any{"type": part}
		if part != "outline" {
			args["slide_id"] = generationSlide
		}
		if result := (pptReadTool{}).Execute(context.Background(), DomainToolInput{ProjectDir: dir, Context: pack, Args: args}); result.Code != CodeResourceNotFound || !strings.Contains(result.Summary, "outline") {
			t.Fatalf("%s did not report its missing outline dependency: %+v", part, result)
		}
	}
	writeGenerationFile(t, dir, ".outline.json", outlineSource)
	for _, part := range []string{"spec", "html"} {
		result := resourceReadToolFailure(fs.ErrNotExist, Resource{Type: "slide", Part: part})
		want := "edit_spec"
		if part == "html" {
			want = "edit_html"
		}
		if !strings.Contains(errorObservation(t, result)["measure"].(string), want) {
			t.Fatalf("missing %s feedback: %+v", part, result)
		}
		resourcePath := model.SpecCollectionPath
		if part == "html" {
			resourcePath = model.SlideHTMLPath(generationSlide)
		}
		source, err := os.ReadFile(filepath.Join(dir, resourcePath))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(dir, resourcePath)); err != nil {
			t.Fatal(err)
		}
		input := DomainToolInput{ProjectDir: dir, Context: pack, Scope: pack.Command.Scope, Args: map[string]any{"type": part, "slide_id": generationSlide}}
		for _, emptyCollection := range []bool{false, true} {
			if emptyCollection {
				if part != "spec" {
					continue
				}
				writeGenerationFile(t, dir, resourcePath, []byte(`{}`))
			}
			missing := (pptReadTool{pack: pack}).Execute(context.Background(), input)
			if missing.OK || missing.Applied || missing.Code != CodeResourceNotFound || missing.Retryable || missing.ObservationMetadata != nil || len(missing.Evidence) != 0 {
				t.Fatalf("uncreated %s was not a missing resource error: %+v", part, missing)
			}
			call := llm.ToolCall{ID: "missing-" + part, Name: "read_resource", Args: input.Args}
			state := &RunState{toolFailures: 1, messages: appendBatchObservations(nil, []llm.ToolCall{call}, "", []ToolResult{missing})}
			recordToolFailures(state, []ToolResult{missing})
			state.rememberResourceVersions(0)
			if state.toolFailures != 2 || len(state.seenVersions) != 0 {
				t.Fatalf("missing %s did not count as a failure or supplied a source version: %+v", part, state)
			}
			input.Messages = state.messages
		}
		input.Args = map[string]any{"type": part, "slide_id": "sli_deleted"}
		if invalid := (pptReadTool{pack: pack}).Execute(context.Background(), input); invalid.OK || invalid.Code != CodeResourceNotFound {
			t.Fatalf("invalid page treated as uncreated %s: %+v", part, invalid)
		}
		writeGenerationFile(t, dir, resourcePath, source)
		if _, err := modelSeenResourceVersion(input, Resource{Type: "slide", Part: part, SlideID: generationSlide}); !errors.Is(err, errResourceUnseen) {
			t.Fatalf("failed read authorized subsequently created %s source: %v", part, err)
		}
		writeGenerationFile(t, dir, ".outline.json", []byte(`{"sections":[]}`))
		input.Args = map[string]any{"type": part, "slide_id": generationSlide}
		if deleted := (pptReadTool{pack: pack}).Execute(context.Background(), input); deleted.OK || deleted.Code != CodeResourceNotFound {
			t.Fatalf("orphaned %s returned for a deleted page: %+v", part, deleted)
		}
		writeGenerationFile(t, dir, ".outline.json", outlineSource)
		if part == "html" {
			for _, html := range []string{"", "raw HTML without a slide stage", strings.Repeat("x", maxPPTContentBytes+1)} {
				writeGenerationFile(t, dir, resourcePath, []byte(html))
				read := (pptReadTool{}).Execute(context.Background(), input)
				if !read.OK || read.Data["content"] != html || read.ObservationMetadata == nil || len(read.ObservationMetadata.Resources) != 1 {
					t.Fatalf("HTML source was limited or validated during read: %+v", read)
				}
			}
			writeGenerationFile(t, dir, resourcePath, source)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, model.SpecCollectionPath), []byte(`{"invalid_slide_key":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	read = (pptReadTool{pack: pack}).Execute(context.Background(), DomainToolInput{ProjectDir: dir, Args: map[string]any{"type": "spec", "slide_id": generationSlide}})
	if read.Code != "RESOURCE_INVALID" {
		t.Fatalf("damaged spec collection treated as absent: %+v", read)
	}
	for _, part := range []string{"manifest", "design"} {
		resourcePath := "." + part + ".json"
		source, err := os.ReadFile(filepath.Join(dir, resourcePath))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(dir, resourcePath)); err != nil {
			t.Fatal(err)
		}
		input := DomainToolInput{ProjectDir: dir, Context: pack, Args: map[string]any{"type": part}}
		if read := (pptReadTool{}).Execute(context.Background(), input); read.Code != CodeResourceNotFound {
			t.Fatalf("missing global resource: %+v", read)
		}
		for _, invalid := range []string{"", "{", "{}"} {
			writeGenerationFile(t, dir, resourcePath, []byte(invalid))
			if read := (pptReadTool{}).Execute(context.Background(), input); read.Code != "RESOURCE_INVALID" {
				t.Fatalf("invalid global JSON returned as readable: %+v", read)
			}
		}
		writeGenerationFile(t, dir, resourcePath, append([]byte(strings.Repeat(" ", maxPPTContentBytes+1)), source...))
		if read := (pptReadTool{}).Execute(context.Background(), input); !read.OK || read.ObservationMetadata == nil {
			t.Fatalf("large valid JSON was rejected: %+v", read)
		}
		writeGenerationFile(t, dir, resourcePath, source)
	}
	if result := resourceReadToolFailure(&fs.PathError{Op: "read", Path: path, Err: fs.ErrPermission}, Resource{Type: "deck", Part: "outline"}); result.Code != model.CodeInternal {
		t.Fatalf("resource I/O failure treated as missing or invalid: %+v", result)
	}
	for _, test := range []struct {
		err  error
		code string
	}{
		{model.NewAgentError("SKILL_NOT_FOUND", "load_skill", errors.New("missing skill")), "SKILL_NOT_FOUND"},
		{model.RepositoryDisabledError("skill", "disabled"), model.CodeResourceNotDisclosed},
		{model.RepositoryContentError("skill", "broken", model.ResourceContentState{ContentState: "invalid", ContentError: "invalid UTF-8"}), model.CodeInternal},
		{&fs.PathError{Op: "read", Path: "SKILL.md", Err: fs.ErrPermission}, model.CodeInternal},
	} {
		result := repositoryLoadFailure("skill", "", test.err)
		if result.Code != test.code || errorObservation(t, result)["measure"] == "" {
			t.Fatalf("repository load failure classification: %+v", result)
		}
	}
}

func TestToolErrorCategoriesAndFlatResourceContract(t *testing.T) {
	readTimeout := &llm.ProviderError{Kind: llm.ErrBadRequest, StatusCode: 401, Phase: "body_read_failed", Cause: context.DeadlineExceeded}
	if err := classifyProviderError(context.Background(), readTimeout); err.Code != "PROVIDER_BAD_REQUEST" || err.Retryable {
		t.Fatalf("reading an authentication error changed retry semantics: %+v", err)
	}
	if err := classifyProviderError(context.Background(), llm.ErrBadToolCall); err.Code != "TOOL_CALL_INVALID" || err.ShouldAutoRetry() {
		t.Fatalf("undecodable protocol response was not distinguished from tool validation: %+v", err)
	}
	for _, code := range []string{
		commandexec.CodeParseInvalid, commandexec.CodeSyntaxDenied, commandexec.CodeNotAllowed,
		commandexec.CodeNotAvailable, commandexec.CodeFlagDenied, model.CodeTargetOutOfScope,
		commandexec.CodePathInvalid, commandexec.CodeExitNonzero,
		commandexec.CodeTimeout, commandexec.CodeOutputLimit,
		"OUTLINE_NOT_INITIALIZED", "PLAN_INVALID", CodeToolArgumentInvalid,
	} {
		def := model.ErrorDefinitionFor(code)
		if def.Code != code || def.Category != model.ErrorRepairable || def.Retryable {
			t.Fatalf("wrong repair category: %+v", def)
		}
	}
	for _, code := range []string{model.CodePersistenceFailed, model.CodeInternal} {
		if def := model.ErrorDefinitionFor(code); def.Code != code || def.Category != model.ErrorTerminal {
			t.Fatalf("infrastructure failure misclassified: %+v", def)
		}
	}
	for _, args := range []map[string]any{{"type": "spec"}, {"type": "outline", "slide_id": "sli_example"}, {"type": map[string]any{"kind": "outline"}}, {"resource": "outline"}, {"type": "outline", "resource": "outline"}} {
		if _, _, err := prepareToolArguments((pptReadTool{}).Schema(), args); err == nil {
			t.Fatalf("accepted invalid resource parameters: %v", args)
		}
	}
}
