package service

import (
	"context"
	"errors"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	"go.uber.org/zap"
)

const renameCorrectionGuidance = "本轮命名结果尚未提交。请仅调用一次 rename_thread：改名时传入 action=rename 和合法 title；保持现名时传入 action=keep 并省略 title。不要附带普通正文。被拒绝的调用不算有效提交。"

func (svc *NamingService) generateRename(ctx context.Context, task renameTask, provider llm.Provider, snapshot string) (action, title string, resultErr error) {
	parallel := false
	policy := &llm.ToolConstraintPolicy{}
	strategy := llm.ToolStrategy{Provider: provider.Name(), Model: provider.Model(), RequiredTool: renameToolName, ParallelToolCalls: &parallel}
	req := llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: llm.TextContent(prompts.MustLoad("command.rename").Body)},
			{Role: llm.RoleUser, Content: llm.TextContent(snapshot)},
		},
		Tools: []llm.ToolSchema{renameThreadToolSchema()}, MaxOutputTokens: 128,
		RequiredTool: renameToolName, ParallelToolCalls: &parallel, ToolConstraintPolicy: policy,
		OnToolStrategy: func(s llm.ToolStrategy) { strategy = s },
	}
	requests, corrections, unsupportedRetries := 0, 0, 0
	defer func() {
		disposition := "accepted"
		if resultErr != nil {
			disposition = "failed"
		}
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(resultErr, context.Canceled) {
			disposition = "canceled_or_superseded"
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			disposition = "timed_out"
		}
		svc.log.Info("naming submission finished", zap.String("thread_id", task.threadID), zap.String("request_id", task.requestID), zap.Int("model_requests", requests), zap.Int("corrections", corrections), zap.Int("unsupported_retries", unsupportedRetries), zap.Int("requests_remaining", renameMaxRequests-requests), zap.String("disposition", disposition), zap.Any("strategy", strategy), zap.Any("execution", llm.ExecutionOf(provider)))
	}()
	for attempt := 0; attempt < renameMaxRequests; attempt++ {
		if err := svc.renameTaskCurrent(ctx, task); err != nil {
			return "", "", err
		}
		requests++
		response, err := provider.Generate(ctx, req)
		if currentErr := svc.renameTaskCurrent(ctx, task); currentErr != nil {
			return "", "", currentErr
		}
		if err != nil {
			canRetry := attempt+1 < renameMaxRequests && policy.Learn(err)
			// No standard response exists here: tool count and has_text are unknown.
			svc.log.Warn("naming provider request failed", zap.String("thread_id", task.threadID), zap.String("request_id", task.requestID), zap.Int("model_request", requests), zap.Any("strategy", strategy), zap.Int("requests_remaining", renameMaxRequests-requests), zap.Bool("will_retry", canRetry), zap.Error(err))
			if !canRetry {
				return "", "", err
			}
			unsupportedRetries++
			continue
		}
		action, title, err = parseRenameResponse(response)
		if err == nil {
			return action, title, nil
		}
		d := llm.SubmissionDiagnostic(response, err)
		d["model_request"], d["corrections"], d["requests_remaining"] = requests, corrections, renameMaxRequests-requests
		canCorrect := attempt+1 < renameMaxRequests && llm.CanReplaySubmission(response)
		d["will_correct"] = canCorrect
		svc.log.Warn("naming protocol violation", zap.String("thread_id", task.threadID), zap.String("request_id", task.requestID), zap.Any("strategy", strategy), zap.Any("execution", llm.ExecutionOf(provider)), zap.Any("diagnostic", d))
		if !canCorrect {
			return "", "", err
		}
		req.Messages = append(req.Messages, llm.Message{Role: llm.RoleAssistant, Content: response.Content, ToolCalls: response.ToolCalls})
		for _, call := range response.ToolCalls {
			req.Messages = append(req.Messages, llm.Message{Role: llm.RoleTool, ToolCallID: call.ID, Content: llm.TextContent(llm.RejectedSubmissionOutput(err, renameCorrectionGuidance))})
		}
		req.Messages = append(req.Messages, llm.Message{Role: llm.RoleUser, Content: llm.TextContent(err.Error() + "\n" + renameCorrectionGuidance), Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "guidance"}})
		req.Continuation = response.Continuation
		corrections++
	}
	return "", "", errors.New("rename request budget exhausted")
}
