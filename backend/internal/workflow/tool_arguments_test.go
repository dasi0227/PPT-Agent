package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestToolArgumentsConvertOnlyDeclaredArraysAndValidateResult(t *testing.T) {
	schema := ToolSchema{Parameters: planProposalParameters()}
	args := map[string]any{"title": "Plan", "content": `["keep this Markdown exactly"]`, "steps": `[{"title":"Page","target_slide_ids":"[\"sli_abc\"]"}]`}
	next, fields, err := prepareToolArguments(schema, args)
	if err != nil || len(fields) != 2 || next["content"] != args["content"] {
		t.Fatalf("normalized=%v fields=%v err=%v", next, fields, err)
	}
	steps := next["steps"].([]any)
	if !reflect.DeepEqual(steps[0].(map[string]any)["target_slide_ids"], []any{"sli_abc"}) {
		t.Fatal("nested array was not converted")
	}
	if _, ok := args["steps"].(string); !ok {
		t.Fatal("preparation mutated original request")
	}
	progress := ToolSchema{Parameters: planProgressParameters()}
	for _, input := range []string{
		`null`, `{}`, `[`, `"[]"`, `[]`, `["completed"]`,
		`[{"step_id":"s1","status":"done"}]`,
		`[{"step_id":"s1","status":"completed","extra":true}]`,
		`[{"step_id":"s1","status":"failed","status":"completed"}]`,
	} {
		if _, _, err := prepareToolArguments(progress, map[string]any{"updates": input}); err == nil {
			t.Fatalf("accepted invalid encoded array: %s", input)
		}
	}
	_, _, err = prepareToolArguments(progress, map[string]any{"updates": "not an array"})
	result := argumentFailure(err)
	var observation map[string]any
	if err := json.Unmarshal([]byte(result.Observation), &observation); err != nil {
		t.Fatal(err)
	}
	if result.Code != CodeToolArgumentInvalid || observation["field"] != "/updates" || observation["expected"] != "array" || observation["actual"] != "string" || observation["category"] != string(model.ErrorAgentRepairable) {
		t.Fatalf("unhelpful repair feedback: %+v", result)
	}
	// Object fields stay strict; the fallback is not a general JSON parser.
	_, _, err = prepareToolArguments((resourceEditTool{name: "edit_design"}).Schema(), map[string]any{"decorations": `{"deck_title":"none"}`})
	if result := argumentFailure(err); result.Data["expected"] != "object" || result.Data["field"] != "/decorations" {
		t.Fatalf("object type error: %+v", result)
	}
	if model.NewAgentError("PLAN_INVALID", "tool_call", nil).Category != model.ErrorAgentRepairable {
		t.Fatal("invalid plans must be repairable rather than terminal")
	}
}

func TestPlanToolsDiscloseOnlyTheCurrentOperation(t *testing.T) {
	for _, mode := range []model.RunMode{model.ModePlan, model.ModeExecute} {
		phase := PhaseExecuting
		if mode == model.ModePlan {
			phase = PhasePlanning
		}
		for _, status := range []PlanStatus{"", PlanAwaitingApproval, PlanActive, PlanCompleted, PlanCanceled} {
			var plan *Plan
			if status != "" {
				plan = &Plan{Status: status}
			}
			schemas := controlSchemas(phase, mode, plan)
			names := schemasByName(schemas)
			wantUpdate := mode == model.ModePlan && status == PlanAwaitingApproval || mode == model.ModeExecute && status == PlanActive
			if names["create_plan"] != (plan == nil) || names["update_plan"] != wantUpdate {
				t.Fatalf("mode=%s status=%s tools=%v", mode, status, names)
			}
			for _, schema := range schemas {
				if schema.Name == "update_plan" {
					if _, exists := schema.Parameters["oneOf"]; exists {
						t.Fatal("plan operation still has competing parameter shapes")
					}
				}
			}
		}
	}
}

func TestArrayFallbackCoversResourceAndControlSchemas(t *testing.T) {
	cases := []struct {
		schema ToolSchema
		args   map[string]any
	}{
		{(resourceEditTool{name: "edit_manifest"}).Schema(), map[string]any{"requirements": `["Include examples"]`}},
		{(resourceEditTool{name: "edit_design"}).Schema(), map[string]any{"layout_preferences": `["Use whitespace"]`}},
		{(resourceEditTool{name: "edit_spec"}).Schema(), map[string]any{"slide_id": "sli_abc", "elements": `[{"type":"text","intent":"Explain"}]`}},
		{(resourceEditTool{name: "patch_html"}).Schema(), map[string]any{"slide_id": "sli_abc", "edits": `[{"old_text":"old","new_text":"new"}]`}},
		{(resourceEditTool{name: "arrange_outline"}).Schema(), map[string]any{"edits": `[{"old_text":"old","new_text":"new"}]`}},
		{(loadComponentTool{}).Schema(), map[string]any{"ids": `["sample"]`}},
	}
	for _, schema := range controlSchemas(PhaseExecuting, model.ModeExecute, nil) {
		switch schema.Name {
		case "ask_user":
			cases = append(cases, struct {
				schema ToolSchema
				args   map[string]any
			}{schema, map[string]any{"questions": `[{"id":"q","title":"Choose","options":"[{\"id\":\"a\",\"label\":\"A\"}]"}]`}})
		case "request_privilege":
			cases = append(cases, struct {
				schema ToolSchema
				args   map[string]any
			}{schema, map[string]any{"add_slide_ids": `["sli_abc"]`, "reason": "Update the page"}})
		}
	}
	for _, test := range cases {
		_, fields, err := prepareToolArguments(test.schema, test.args)
		if err != nil || len(fields) == 0 {
			t.Fatalf("%s: fields=%v err=%v", test.schema.Name, fields, err)
		}
	}
}

type repairedPlanAgent struct{ turns int }

func (a *repairedPlanAgent) Next(_ context.Context, request AgentRequest) (AgentResponse, error) {
	a.turns++
	names := schemasByName(request.Tools)
	switch a.turns {
	case 1:
		return toolCall("create", "create_plan", map[string]any{"title": "Check", "content": "Check current page", "steps": `[{"title":"Check page"}]`}), nil
	case 2:
		if names["create_plan"] || !names["update_plan"] {
			return AgentResponse{}, errors.New("active plan tool disclosure")
		}
		return toolCall("bad", "update_plan", map[string]any{"updates": "not JSON"}), nil
	case 3:
		if request.Plan.Steps[0].Status != PlanStepPending || !strings.Contains(transcriptText(request.Messages), "TOOL_ARGUMENT_INVALID") {
			return AgentResponse{}, errors.New("invalid call changed state or feedback missing")
		}
		return toolCall("overwrite", "create_plan", map[string]any{"title": "Overwrite", "content": "Overwrite", "steps": []any{map[string]any{"title": "Skip"}}}), nil
	case 4:
		if request.Plan.Title != "Check" {
			return AgentResponse{}, errors.New("active plan was replaced")
		}
		return finishCall("unfinished"), nil
	case 5:
		if !strings.Contains(transcriptText(request.Messages), "PLAN_NOT_COMPLETE") {
			return AgentResponse{}, errors.New("completion did not enforce unfinished plan")
		}
		updates, _ := json.Marshal([]map[string]string{{"step_id": request.Plan.Steps[0].ID, "status": "completed"}})
		return toolCall("repair", "update_plan", map[string]any{"updates": string(updates)}), nil
	default:
		if names["create_plan"] || names["update_plan"] || request.Plan.Status != PlanCompleted {
			return AgentResponse{}, errors.New("completed plan disclosure or status")
		}
		return finishCall("done"), nil
	}
}

func TestRuntimeRepairsEncodedPlanProgressAndFinishes(t *testing.T) {
	agent := &repairedPlanAgent{}
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "repair-plan", ProjectDir: testProject(t, ArtifactSlideSpec),
		Context:     testPack(model.ModeExecute, model.ScopeCurrentPage, false, "检查当前页"),
		DomainTools: fakeProvider{kind: ArtifactSlideSpec}, SemanticReviews: acceptingReviewer{},
	})
	if outcome.Status != StatusCompleted || agent.turns != 6 {
		t.Fatalf("outcome=%+v turns=%d", outcome, agent.turns)
	}
}

type argumentCaptureTool struct{ received map[string]any }

func (*argumentCaptureTool) Schema() ToolSchema {
	return ToolSchema{Name: "capture_arrays", Parameters: objectSchema([]string{"edits"}, map[string]any{"edits": textEditsSchema()})}
}
func (t *argumentCaptureTool) Execute(_ context.Context, input DomainToolInput) ToolResult {
	t.received = input.Args
	return SuccessfulToolResult("accepted")
}

func TestDomainToolReceivesNormalizedArgumentsWithoutChangingText(t *testing.T) {
	tool := &argumentCaptureTool{}
	registry := NewToolRegistry()
	if err := registry.Register(tool, true, CapabilityRead, RiskLow, PhaseExecuting); err != nil {
		t.Fatal(err)
	}
	pack := testPack(model.ModeExecute, model.ScopeCurrentPage, false, "检查")
	input := DomainToolInput{Context: pack, Scope: pack.Command.Scope, Mode: model.ModeExecute, Phase: PhaseExecuting}
	args := map[string]any{"edits": `[{"old_text":"[1,2]","new_text":"[3,4]"}]`}
	result := registry.Execute(context.Background(), map[string]bool{"capture_arrays": true}, "capture_arrays", args, input)
	if !result.OK || tool.received["edits"].([]any)[0].(map[string]any)["old_text"] != "[1,2]" {
		t.Fatalf("result=%+v args=%v", result, tool.received)
	}
	tool.received = nil
	result = registry.Execute(context.Background(), map[string]bool{"capture_arrays": true}, "capture_arrays", map[string]any{"edits": `[{}]`}, input)
	if result.Code != CodeToolArgumentInvalid || tool.received != nil {
		t.Fatalf("invalid request reached tool: %+v", result)
	}
}

type planTestAgentFunc func(context.Context, AgentRequest) (AgentResponse, error)

func (f planTestAgentFunc) Next(ctx context.Context, request AgentRequest) (AgentResponse, error) {
	return f(ctx, request)
}

type reviseOncePrompter struct{ approvingPrompter }

func (p *reviseOncePrompter) AskPlanApproval(ctx context.Context, request model.PlanApprovalRequestedPayload) (model.PlanApprovalAnswer, error) {
	if p.calls == 0 {
		p.calls++
		return model.PlanApprovalAnswer{InteractionID: request.InteractionID, PlanID: request.Plan.PlanID, Decision: "revise", Feedback: "改成简短检查"}, nil
	}
	return p.approvingPrompter.AskPlanApproval(ctx, request)
}

func TestRevisedPlanReturnsToApprovalBeforeExecution(t *testing.T) {
	turns := 0
	prompter := &reviseOncePrompter{}
	agent := planTestAgentFunc(func(_ context.Context, req AgentRequest) (AgentResponse, error) {
		turns++
		switch turns {
		case 1:
			return toolCall("create", "create_plan", map[string]any{"title": "检查", "content": "检查当前页", "steps": []any{map[string]any{"title": "检查"}}}), nil
		case 2:
			if req.Mode != model.ModePlan || schemasByName(req.Tools)["create_plan"] {
				return AgentResponse{}, errors.New("revision must expose only update_plan")
			}
			return toolCall("revise", "update_plan", map[string]any{"title": "简短检查", "content": "简短检查当前页", "steps": `[{"title":"检查"}]`}), nil
		case 3:
			if req.Mode != model.ModeExecute || prompter.calls != 2 || req.Plan.Title != "简短检查" {
				return AgentResponse{}, errors.New("revised plan was not approved before execution")
			}
			return toolCall("complete", "update_plan", map[string]any{"updates": []any{map[string]any{"step_id": req.Plan.Steps[0].ID, "status": "completed"}}}), nil
		default:
			return finishCall("done"), nil
		}
	})
	outcome := NewRuntime(agent).Run(context.Background(), RuntimeInput{
		RunID: "revise-plan", ProjectDir: testProject(t, ArtifactSlideSpec), Prompter: prompter,
		Context:     testPack(model.ModePlan, model.ScopeCurrentPage, false, "检查当前页"),
		DomainTools: fakeProvider{kind: ArtifactSlideSpec}, SemanticReviews: acceptingReviewer{},
	})
	if outcome.Status != StatusCompleted || turns != 4 || prompter.calls != 2 {
		t.Fatalf("outcome=%+v turns=%d approvals=%d", outcome, turns, prompter.calls)
	}
}
