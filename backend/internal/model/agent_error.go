package model

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type ErrorCategory string

const (
	ErrorTransient          ErrorCategory = "transient"
	ErrorAgentRepairable    ErrorCategory = "agent_repairable"
	ErrorUserActionRequired ErrorCategory = "user_action_required"
	ErrorConflict           ErrorCategory = "conflict"
	ErrorCanceled           ErrorCategory = "canceled"
	ErrorTerminal           ErrorCategory = "terminal"
)

type ErrorResource struct {
	Type    string `json:"type"`
	SlideID string `json:"slide_id,omitempty"`
	Part    string `json:"part"`
}

type ErrorDefinition struct {
	Code         string
	Category     ErrorCategory
	SafeMessage  string
	ModelMessage string
	Retryable    bool
	HTTPStatus   int
}

var errorDefinitions = map[string]ErrorDefinition{
	"BAD_REQUEST":                    {Code: "BAD_REQUEST", Category: ErrorUserActionRequired, SafeMessage: "请求内容不合法。", ModelMessage: "Correct the request and try again.", HTTPStatus: 400},
	"NOT_FOUND":                      {Code: "NOT_FOUND", Category: ErrorUserActionRequired, SafeMessage: "未找到请求的内容。", ModelMessage: "Choose a resource that exists in the current project.", HTTPStatus: 404},
	"CONFLICT":                       {Code: "CONFLICT", Category: ErrorConflict, SafeMessage: "请求与当前状态冲突，请刷新后重试。", ModelMessage: "Refresh authoritative state before the next action.", HTTPStatus: 409},
	"VALIDATION_FAILED":              {Code: "VALIDATION_FAILED", Category: ErrorAgentRepairable, SafeMessage: "内容未通过校验。", ModelMessage: "Correct the reported validation issue before retrying.", HTTPStatus: 422},
	"RUN_NOT_FOUND":                  {Code: "RUN_NOT_FOUND", Category: ErrorUserActionRequired, SafeMessage: "未找到当前任务。", ModelMessage: "The requested run does not exist.", HTTPStatus: 404},
	"SLIDE_NOT_FOUND":                {Code: "SLIDE_NOT_FOUND", Category: ErrorUserActionRequired, SafeMessage: "未找到指定页面。", ModelMessage: "Choose a slide_id that belongs to the current project.", HTTPStatus: 400},
	"SPEC_REVISION_CONFLICT":         {Code: "SPEC_REVISION_CONFLICT", Category: ErrorConflict, SafeMessage: "设计稿已被更新，请刷新后重试。", ModelMessage: "Read the current spec revision before applying another patch.", HTTPStatus: 409},
	"SPEC_REFERENCE_BROKEN":          {Code: "SPEC_REFERENCE_BROKEN", Category: ErrorAgentRepairable, SafeMessage: "设计稿引用了不存在的内容。", ModelMessage: "Correct the broken slide or asset reference.", HTTPStatus: 422},
	"SPEC_INVALID":                   {Code: "SPEC_INVALID", Category: ErrorAgentRepairable, SafeMessage: "设计稿未通过格式检查。", ModelMessage: "Correct the reported spec validation issue.", HTTPStatus: 422},
	"INVALID_TARGET":                 {Code: "INVALID_TARGET", Category: ErrorUserActionRequired, SafeMessage: "任务目标不合法。", ModelMessage: "Correct target.artifact, target.level, and slide_id.", HTTPStatus: 422},
	"RUN_TARGET_UNSUPPORTED":         {Code: "RUN_TARGET_UNSUPPORTED", Category: ErrorUserActionRequired, SafeMessage: "当前任务类型暂不支持。", ModelMessage: "Choose a supported PPT target.", HTTPStatus: 422},
	"MODEL_PROFILE_NOT_FOUND":        {Code: "MODEL_PROFILE_NOT_FOUND", Category: ErrorUserActionRequired, SafeMessage: "所选模型已不可用，请重新选择。", ModelMessage: "Choose an exact model profile returned by the server.", HTTPStatus: 422},
	"MODEL_CAPABILITY_MISMATCH":      {Code: "MODEL_CAPABILITY_MISMATCH", Category: ErrorUserActionRequired, SafeMessage: "所选模型不具备当前任务所需能力，请重新选择。", ModelMessage: "Choose a model profile that satisfies the required capability.", HTTPStatus: 422},
	"MODEL_PROVIDER_UNSUPPORTED":     {Code: "MODEL_PROVIDER_UNSUPPORTED", Category: ErrorTerminal, SafeMessage: "模型服务配置不受当前版本支持。", ModelMessage: "An administrator must correct the configured provider.", HTTPStatus: 500},
	"IDEMPOTENCY_KEY_REUSED":         {Code: "IDEMPOTENCY_KEY_REUSED", Category: ErrorConflict, SafeMessage: "该请求标识已用于其他内容，请重新发送。", ModelMessage: "Generate a new client request identifier for the changed request.", HTTPStatus: 409},
	"RUN_WAITING_FOR_ANSWER":         {Code: "RUN_WAITING_FOR_ANSWER", Category: ErrorConflict, SafeMessage: "当前任务正在等待问题回答，请先回答问题。", ModelMessage: "Answer the pending ask_user question instead of steering.", HTTPStatus: 409},
	"RUN_CANCELING":                  {Code: "RUN_CANCELING", Category: ErrorCanceled, SafeMessage: "当前任务正在取消，无法再追加要求。", ModelMessage: "Wait for the canceled terminal state.", HTTPStatus: 409},
	"RUN_NOT_STEERABLE":              {Code: "RUN_NOT_STEERABLE", Category: ErrorConflict, SafeMessage: "当前任务已进入完成阶段，请作为新要求发送。", ModelMessage: "Start a new run with this instruction.", HTTPStatus: 409},
	"RUN_CANCELED":                   {Code: "RUN_CANCELED", Category: ErrorCanceled, SafeMessage: "任务已取消。", ModelMessage: "The run was canceled; do not retry this action.", HTTPStatus: 409},
	"RUN_FAILED":                     {Code: "RUN_FAILED", Category: ErrorTerminal, SafeMessage: "任务未能完成。", ModelMessage: "Stop the run and inspect the internal trace.", HTTPStatus: 500},
	"LOCK_TIMEOUT":                   {Code: "LOCK_TIMEOUT", Category: ErrorTransient, SafeMessage: "等待项目执行资源超时，请稍后重试。", ModelMessage: "Retry the run after the project execution slot becomes available.", Retryable: true, HTTPStatus: 503},
	"RESOURCE_INVALID":               {Code: "RESOURCE_INVALID", Category: ErrorAgentRepairable, SafeMessage: "PPT 资源地址不合法。", ModelMessage: "Use one disclosed deck or slide Resource.", HTTPStatus: 422},
	"RESOURCE_NOT_DISCLOSED":         {Code: "RESOURCE_NOT_DISCLOSED", Category: ErrorAgentRepairable, SafeMessage: "当前步骤不能使用该资源。", ModelMessage: "Use only resources and tools disclosed for this turn.", HTTPStatus: 422},
	"TARGET_OUT_OF_SCOPE":            {Code: "TARGET_OUT_OF_SCOPE", Category: ErrorAgentRepairable, SafeMessage: "目标超出当前任务范围。", ModelMessage: "Keep the operation inside the current WorkSpec scope.", HTTPStatus: 409},
	"RESOURCE_NOT_FOUND":             {Code: "RESOURCE_NOT_FOUND", Category: ErrorAgentRepairable, SafeMessage: "未找到所需 PPT 内容。", ModelMessage: "Read the outline and choose an existing Resource.", HTTPStatus: 404},
	"CONTENT_TOO_LARGE":              {Code: "CONTENT_TOO_LARGE", Category: ErrorAgentRepairable, SafeMessage: "PPT 内容超过处理上限。", ModelMessage: "Reduce the resource size before retrying.", HTTPStatus: 422},
	"CONTENT_INVALID":                {Code: "CONTENT_INVALID", Category: ErrorAgentRepairable, SafeMessage: "PPT 内容未通过格式检查。", ModelMessage: "Correct the reported JSON Pointer or HTML checks.", HTTPStatus: 422},
	"EDIT_ANCHOR_NOT_FOUND":          {Code: "EDIT_ANCHOR_NOT_FOUND", Category: ErrorAgentRepairable, SafeMessage: "未找到要替换的内容。", ModelMessage: "Call read_ppt and use an exact current anchor.", HTTPStatus: 422},
	"EDIT_ANCHOR_AMBIGUOUS":          {Code: "EDIT_ANCHOR_AMBIGUOUS", Category: ErrorAgentRepairable, SafeMessage: "要替换的内容不唯一。", ModelMessage: "Call read_ppt and choose a longer unique anchor.", HTTPStatus: 422},
	"REVISION_CONFLICT":              {Code: "REVISION_CONFLICT", Category: ErrorConflict, SafeMessage: "PPT 已被其他操作更新，请重新读取后再试。", ModelMessage: "Read the current Resource revision before making another edit.", HTTPStatus: 409},
	"RENDER_FAILED":                  {Code: "RENDER_FAILED", Category: ErrorAgentRepairable, SafeMessage: "页面视觉检查未通过。", ModelMessage: "Fix the reported HTML or render diagnostics and render again.", HTTPStatus: 422},
	"TARGET_ALREADY_EXISTS":          {Code: "TARGET_ALREADY_EXISTS", Category: ErrorAgentRepairable, SafeMessage: "目标内容已经存在。", ModelMessage: "Read the current Resource and edit it instead of creating it again.", HTTPStatus: 409},
	"RUN_SESSION_REQUIRED":           {Code: "RUN_SESSION_REQUIRED", Category: ErrorAgentRepairable, SafeMessage: "当前操作需要活跃的写入会话。", ModelMessage: "Perform the write inside the active run session.", HTTPStatus: 422},
	"RUN_SESSION_INVALID":            {Code: "RUN_SESSION_INVALID", Category: ErrorAgentRepairable, SafeMessage: "当前写入会话状态无效。", ModelMessage: "Refresh the current run session before attempting to finish.", HTTPStatus: 422},
	"INVALID_CONTROL_CALL":           {Code: "INVALID_CONTROL_CALL", Category: ErrorAgentRepairable, SafeMessage: "Agent 控制动作不适用于当前阶段。", ModelMessage: "Choose a runtime control action disclosed for the current phase.", HTTPStatus: 422},
	"SCOPE_EXPANSION_REQUIRED":       {Code: "SCOPE_EXPANSION_REQUIRED", Category: ErrorAgentRepairable, SafeMessage: "任务范围需要先更新计划。", ModelMessage: "Update the plan in the same ReAct loop before expanding the work scope.", HTTPStatus: 422},
	"DEPENDENCY_FAILED":              {Code: "DEPENDENCY_FAILED", Category: ErrorAgentRepairable, SafeMessage: "前序操作失败，后续操作未执行。", ModelMessage: "Repair the first failed call before retrying dependent calls.", HTTPStatus: 422},
	"COMPLETION_GATE_BLOCKED":        {Code: "COMPLETION_GATE_BLOCKED", Category: ErrorAgentRepairable, SafeMessage: "任务尚未满足完成条件。", ModelMessage: "Perform the required next actions reported by the completion gate.", HTTPStatus: 422},
	"CONSECUTIVE_TOOL_ERRORS":        {Code: "CONSECUTIVE_TOOL_ERRORS", Category: ErrorTerminal, SafeMessage: "连续修正未成功，任务已停止。", ModelMessage: "Stop after the bounded repair budget is exhausted.", HTTPStatus: 500},
	"COMPLETION_REJECTED_REPEATEDLY": {Code: "COMPLETION_REJECTED_REPEATEDLY", Category: ErrorTerminal, SafeMessage: "多次检查仍未满足完成条件，任务已停止。", ModelMessage: "Stop after repeated identical completion-gate rejections.", HTTPStatus: 500},
	"CONTEXT_BUDGET_EXCEEDED":        {Code: "CONTEXT_BUDGET_EXCEEDED", Category: ErrorTerminal, SafeMessage: "上下文超过处理上限。", ModelMessage: "Reduce context size without discarding the latest screenshot bindings.", HTTPStatus: 500},
	"PROVIDER_UNAVAILABLE":           {Code: "PROVIDER_UNAVAILABLE", Category: ErrorTransient, SafeMessage: "模型服务暂时不可用，正在尝试恢复。", ModelMessage: "Retry the provider request with bounded backoff.", Retryable: true, HTTPStatus: 503},
	"RENDER_WORKER_UNAVAILABLE":      {Code: "RENDER_WORKER_UNAVAILABLE", Category: ErrorTransient, SafeMessage: "页面渲染服务暂时不可用。", ModelMessage: "Restart the render worker and retry once.", Retryable: true, HTTPStatus: 503},
	"COMMIT_FAILED":                  {Code: "COMMIT_FAILED", Category: ErrorTerminal, SafeMessage: "修改未能安全保存，请重新发起任务。", ModelMessage: "Do not retry an unconfirmed commit without querying its idempotency record.", HTTPStatus: 500},
	"RUNTIME_BUDGET_EXCEEDED":        {Code: "RUNTIME_BUDGET_EXCEEDED", Category: ErrorTerminal, SafeMessage: "运行达到轮次或时长上限，已生成的内容已保留，可继续推进。", ModelMessage: "Stop the run; direct-written products are already persisted and can be resumed.", HTTPStatus: 500},
	"AGENT_FAILED":                   {Code: "AGENT_FAILED", Category: ErrorTerminal, SafeMessage: "Agent 暂时无法继续，请稍后重试。", ModelMessage: "Stop the run and preserve the internal cause in trace only.", HTTPStatus: 500},
	"INTERNAL":                       {Code: "INTERNAL", Category: ErrorTerminal, SafeMessage: "服务暂时无法完成请求。", ModelMessage: "Stop and inspect the internal trace.", HTTPStatus: 500},
}

var traceSensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:provider[_ ]?body|response[_ ]?body|body)\s*[:=].*$`),
	regexp.MustCompile(`(?i)\bauthorization["']?\s*[:=]\s*["']?(?:bearer\s+)?[^"'\s,;}]+["']?`),
	regexp.MustCompile(`(?i)\b(?:api[_-]?key|key|token|secret)["']?\s*[:=]\s*["']?[^"'\s,;}]+["']?`),
	regexp.MustCompile(`(?i)\bsk-[a-z0-9._-]+\b`),
}

type AgentError struct {
	Code         string
	Category     ErrorCategory
	Operation    string
	Resource     *ErrorResource
	CallID       string
	Retryable    bool
	SafeMessage  string
	ModelMessage string
	Details      map[string]any
	Cause        error
}

func NewAgentError(code, operation string, cause error) *AgentError {
	def, ok := errorDefinitions[code]
	if !ok {
		def = errorDefinitions["INTERNAL"]
		def.Code = code
	}
	return &AgentError{
		Code: def.Code, Category: def.Category, Operation: operation,
		Retryable: def.Retryable, SafeMessage: def.SafeMessage,
		ModelMessage: def.ModelMessage, Details: map[string]any{}, Cause: cause,
	}
}

func (e *AgentError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.Cause)
	}
	return e.Code
}

func (e *AgentError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func AsAgentError(err error, fallbackCode, operation string) *AgentError {
	var agentErr *AgentError
	if errors.As(err, &agentErr) {
		return agentErr
	}
	return NewAgentError(fallbackCode, operation, err)
}

func ErrorDefinitionFor(code string) ErrorDefinition {
	if def, ok := errorDefinitions[code]; ok {
		return def
	}
	return errorDefinitions["INTERNAL"]
}

func RegisteredErrorCodes() []string {
	out := make([]string, 0, len(errorDefinitions))
	for code := range errorDefinitions {
		out = append(out, code)
	}
	return out
}

func (e *AgentError) ShouldAutoRetry() bool {
	return e != nil && e.Category == ErrorTransient && e.Retryable
}

func (e *AgentError) Public() *PublicError {
	if e == nil {
		return nil
	}
	message := strings.TrimSpace(e.SafeMessage)
	if message == "" {
		message = errorDefinitions["INTERNAL"].SafeMessage
	}
	return &PublicError{Code: e.Code, Message: message, Retryable: e.Retryable}
}

func (e *AgentError) ModelObservation() map[string]any {
	if e == nil {
		return nil
	}
	out := map[string]any{
		"ok": false, "code": e.Code, "category": e.Category,
		"reason": e.ModelMessage, "retryable": e.ShouldAutoRetry(),
	}
	if e.Operation != "" {
		out["operation"] = e.Operation
	}
	if e.Resource != nil {
		out["resource"] = e.Resource
	}
	if e.CallID != "" {
		out["call_id"] = e.CallID
	}
	for key, value := range e.Details {
		out[key] = value
	}
	return out
}

func (e *AgentError) TraceProjection() map[string]any {
	if e == nil {
		return nil
	}
	out := e.ModelObservation()
	out["safe_message"] = e.SafeMessage
	if e.Cause != nil {
		out["cause"] = sanitizeTraceCause(e.Cause.Error())
	}
	return out
}

func sanitizeTraceCause(value string) string {
	for _, pattern := range traceSensitivePatterns {
		value = pattern.ReplaceAllString(value, "[REDACTED]")
	}
	return value
}

func (e *AgentError) HTTPStatus() int {
	if e == nil {
		return 500
	}
	return ErrorDefinitionFor(e.Code).HTTPStatus
}
