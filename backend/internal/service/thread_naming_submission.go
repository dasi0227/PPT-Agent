package service

import (
	"context"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	"go.uber.org/zap"
)

const renameCorrectionGuidance = "本轮命名结果尚未提交。请仅调用一次 rename_thread：改名时传入 action=rename 和合法 title；保持现名时传入 action=keep 并省略 title。不要附带普通正文。被拒绝的调用不算有效提交。"

func (svc *NamingService) generateRename(ctx context.Context, task renameTask, provider llm.Provider, snapshot string) (action, title string, resultErr error) {
	session := llm.NewSubmissionSession("rename", 128)
	session.Check = func(ctx context.Context) error { return svc.renameTaskCurrent(ctx, task) }
	session.Diagnose = func(d map[string]any) {
		message := "naming submission diagnostic"
		switch d["event"] {
		case "protocol_violation":
			message = "naming protocol violation"
		case "provider_failure":
			message = "naming provider request failed"
		case "finished":
			message = "naming submission finished"
		}
		svc.log.Info(message, zap.String("thread_id", task.threadID), zap.String("request_id", task.requestID), zap.Any("diagnostic", d))
	}
	_, resultErr = session.Generate(ctx, provider, llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: llm.TextContent(prompts.MustLoad("command.rename").Body)},
			{Role: llm.RoleUser, Content: llm.TextContent(snapshot)},
		}, Tools: []llm.ToolSchema{renameThreadToolSchema()}, MaxOutputTokens: 128,
	}, func(response llm.GenerateResponse) error {
		var err error
		action, title, err = parseRenameResponse(response)
		return err
	}, renameCorrectionGuidance)
	return action, title, resultErr
}
