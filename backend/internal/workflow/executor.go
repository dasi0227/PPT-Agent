package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

type EventEmitter interface {
	Emit(model.EventType, any)
}

type StepInput struct {
	State       WorkflowState
	Context     contextengine.ContextPack
	Step        WorkflowStep
	Selection   ToolSelection
	Transaction *Transaction
	Emitter     EventEmitter
}

type StepResult struct {
	Summary   string        `json:"summary"`
	Artifacts []ArtifactRef `json:"artifacts"`
	Issues    []Issue       `json:"issues"`
	Retryable bool          `json:"retryable"`
}

type StepExecutor interface {
	Execute(context.Context, StepInput) (StepResult, error)
}

type CognitiveExecutor struct {
	Client llm.Client
}

func (e CognitiveExecutor) Execute(ctx context.Context, input StepInput) (StepResult, error) {
	if input.Step.Kind == StepAnalyze && input.State.Plan.Operation == OperationConsult {
		return e.consult(ctx, input)
	}
	if input.Step.Kind == StepAnalyze {
		return StepResult{
			Summary:   "Context and target constraints analyzed",
			Artifacts: []ArtifactRef{}, Issues: []Issue{},
		}, nil
	}
	if e.Client == nil {
		return deterministicStep(input)
	}
	system, user := contextengine.CompileForRunner(&input.Context,
		stepSystemPrompt(input), input.Step.Instruction)
	messages := []llm.Message{{Role: llm.RoleSystem, Content: system}, {Role: llm.RoleUser, Content: user}}
	schemas := make([]llm.ToolSchema, 0, len(input.Selection.Descriptors))
	for _, schema := range input.Selection.Schemas() {
		schemas = append(schemas, llm.ToolSchema{
			Name: schema.Name, Description: schema.Description, Parameters: schema.Parameters,
		})
	}
	artifacts := []ArtifactRef{}
	issues := []Issue{}
	failures := 0
	maxTurns := input.State.Plan.Budget.MaxStepTurns
	if maxTurns <= 0 {
		maxTurns = DefaultBudget().MaxStepTurns
	}
	for turn := 1; turn <= maxTurns; turn++ {
		if err := ctx.Err(); err != nil {
			return StepResult{}, err
		}
		response, err := e.Client.CallTool(ctx, llm.ToolCallRequest{Messages: messages, Tools: schemas})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return StepResult{}, err
			}
			break
		}
		if response.ToolCall == nil {
			if strings.TrimSpace(response.Text) != "" && len(artifacts) == 0 {
				if input.Step.Kind == StepAnalyze {
					return StepResult{Summary: response.Text, Artifacts: artifacts, Issues: issues}, nil
				}
				break
			}
			if len(artifacts) > 0 {
				return StepResult{Summary: response.Text, Artifacts: artifacts, Issues: issues}, nil
			}
			break
		}
		call := *response.ToolCall
		if call.ID == "" {
			call.ID = fmt.Sprintf("%s-%d", input.Step.ID, turn)
		}
		if input.Emitter != nil {
			input.Emitter.Emit(model.EventToolCalled, ToolEvent{
				EventEnvelope: envelope(input.State), CallID: call.ID, Tool: call.Name, Args: call.Args,
			})
		}
		result := input.Selection.Execute(ctx, call.Name, call.Args, input.Transaction)
		if input.Emitter != nil {
			input.Emitter.Emit(model.EventToolCompleted, ToolEvent{
				EventEnvelope: envelope(input.State), CallID: call.ID, Tool: call.Name,
				OK: result.OK, Summary: result.Summary, Artifacts: result.Artifacts, Issues: result.Issues,
			})
		}
		messages = append(messages, llm.Message{
			Role: llm.RoleAssistant, Content: "",
			ToolCalls: []llm.ToolCall{call},
		})
		observation, _ := json.Marshal(result)
		messages = append(messages, llm.Message{Role: llm.RoleTool, ToolCallID: call.ID, Content: string(observation)})
		if result.OK {
			failures = 0
			artifacts = appendUniqueArtifacts(artifacts, result.Artifacts...)
		} else {
			failures++
			issues = append(issues, result.Issues...)
			if !result.Retryable || failures >= input.State.Plan.Budget.MaxToolFailures {
				return StepResult{Summary: result.Summary, Artifacts: artifacts, Issues: issues, Retryable: false}, fmt.Errorf("STEP_FAILED: %s", result.Summary)
			}
		}
		if call.Name == "finish_step" {
			return StepResult{Summary: result.Summary, Artifacts: artifacts, Issues: issues}, nil
		}
	}
	if len(artifacts) > 0 {
		return StepResult{Summary: "Step completed with staged artifacts", Artifacts: artifacts, Issues: issues}, nil
	}
	return deterministicStep(input)
}

func (e CognitiveExecutor) consult(ctx context.Context, input StepInput) (StepResult, error) {
	if e.Client == nil {
		return StepResult{Summary: "Consultation completed from the assembled context", Artifacts: []ArtifactRef{}, Issues: []Issue{}}, nil
	}
	system, user := contextengine.CompileForRunner(&input.Context,
		"You are a presentation consultant. Answer from the assembled context. Do not propose or perform file writes.", input.Step.Instruction)
	response, err := e.Client.Chat(ctx, llm.ChatRequest{Messages: []llm.Message{
		{Role: llm.RoleSystem, Content: system}, {Role: llm.RoleUser, Content: user},
	}})
	if err != nil {
		return StepResult{}, err
	}
	return StepResult{Summary: response.Content, Artifacts: []ArtifactRef{}, Issues: []Issue{}}, nil
}

func stepSystemPrompt(input StepInput) string {
	names := make([]string, 0, len(input.Selection.Descriptors))
	for _, descriptor := range input.Selection.Descriptors {
		names = append(names, descriptor.Name)
	}
	return fmt.Sprintf(
		"You are executing exactly one constrained workflow step. Step=%s kind=%s. "+
			"Only use the disclosed tools (%s), only touch declared targets, and finish after staging the required artifact. "+
			"Never claim a write without a successful staged write tool result.",
		input.Step.ID, input.Step.Kind, strings.Join(names, ", "),
	)
}

func appendUniqueArtifacts(existing []ArtifactRef, values ...ArtifactRef) []ArtifactRef {
	seen := map[string]bool{}
	for _, ref := range existing {
		seen[ref.Key()] = true
	}
	for _, ref := range values {
		if !seen[ref.Key()] {
			existing = append(existing, ref)
			seen[ref.Key()] = true
		}
	}
	return existing
}
