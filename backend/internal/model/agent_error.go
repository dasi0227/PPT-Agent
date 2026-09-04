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
	"INVALID_SCOPE":                  {Code: "INVALID_SCOPE", Category: ErrorUserActionRequired, SafeMessage: "任务范围不合法。", ModelMessage: "Correct scope.artifact, scope.level, and slide_id.", HTTPStatus: 422},
	"RUN_SCOPE_UNSUPPORTED":          {Code: "RUN_SCOPE_UNSUPPORTED", Category: ErrorUserActionRequired, SafeMessage: "当前任务范围暂不支持。", ModelMessage: "Choose a supported PPT scope.", HTTPStatus: 422},
	"MODEL_PROFILE_NOT_FOUND":        {Code: "MODEL_PROFILE_NOT_FOUND", Category: ErrorUserActionRequired, SafeMessage: "所选模型已不可用，请重新选择。", ModelMessage: "Choose an exact model profile returned by the server.", HTTPStatus: 422},
	"MODEL_CAPABILITY_MISMATCH":      {Code: "MODEL_CAPABILITY_MISMATCH", Category: ErrorUserActionRequired, SafeMessage: "所选模型不具备当前任务所需能力，请重新选择。", ModelMessage: "Choose a model profile that satisfies the required capability.", HTTPStatus: 422},
	"MODEL_PROVIDER_UNSUPPORTED":     {Code: "MODEL_PROVIDER_UNSUPPORTED", Category: ErrorTerminal, SafeMessage: "模型服务配置不受当前版本支持。", ModelMessage: "An administrator must correct the configured provider.", HTTPStatus: 500},
	"SKILL_SELECTION_INVALID":        {Code: "SKILL_SELECTION_INVALID", Category: ErrorUserActionRequired, SafeMessage: "技能选择不合法，请重新选择。", ModelMessage: "Choose at most three unique skills returned by the server.", HTTPStatus: 422},
	"SKILL_NOT_FOUND":                {Code: "SKILL_NOT_FOUND", Category: ErrorUserActionRequired, SafeMessage: "所选技能已不可用，请重新选择。", ModelMessage: "Choose an exact skill id returned by the server.", HTTPStatus: 422},
	"COMPONENT_SELECTION_INVALID":    {Code: "COMPONENT_SELECTION_INVALID", Category: ErrorUserActionRequired, SafeMessage: "组件引用不合法，请重新选择。", ModelMessage: "Choose at most eight component names returned by the server.", HTTPStatus: 400},
	"COMPONENT_NOT_FOUND":            {Code: "COMPONENT_NOT_FOUND", Category: ErrorUserActionRequired, SafeMessage: "引用的组件已不存在或已改名，请重新选择。", ModelMessage: "Choose an exact enabled component name returned by the server.", HTTPStatus: 404},
	"COMPONENT_NAME_AMBIGUOUS":       {Code: "COMPONENT_NAME_AMBIGUOUS", Category: ErrorConflict, SafeMessage: "存在重名组件，请先在组件仓库中重命名。", ModelMessage: "Rename duplicate enabled components before retrying.", HTTPStatus: 409},
	"COMPONENT_DISABLED":             {Code: "COMPONENT_DISABLED", Category: ErrorConflict, SafeMessage: "引用的组件已被禁用，请重新选择。", ModelMessage: "Enable the component or choose another enabled component.", HTTPStatus: 409},
	"PAGE_SELECTION_INVALID":         {Code: "PAGE_SELECTION_INVALID", Category: ErrorUserActionRequired, SafeMessage: "页面引用不合法，请重新选择。", ModelMessage: "Choose at most eight stable slide_ids from the current outline.", HTTPStatus: 400},
	"IDEMPOTENCY_KEY_REUSED":         {Code: "IDEMPOTENCY_KEY_REUSED", Category: ErrorConflict, SafeMessage: "该请求标识已用于其他内容，请重新发送。", ModelMessage: "Generate a new client request identifier for the changed request.", HTTPStatus: 409},
	"RUN_WAITING_FOR_ANSWER":         {Code: "RUN_WAITING_FOR_ANSWER", Category: ErrorConflict, SafeMessage: "当前任务正在等待问题回答，请先回答问题。", ModelMessage: "Answer the pending ask_user question instead of steering.", HTTPStatus: 409},
	"RUN_CANCELING":                  {Code: "RUN_CANCELING", Category: ErrorCanceled, SafeMessage: "当前任务正在取消，无法再追加要求。", ModelMessage: "Wait for the canceled terminal state.", HTTPStatus: 409},
	"RUN_NOT_STEERABLE":              {Code: "RUN_NOT_STEERABLE", Category: ErrorConflict, SafeMessage: "当前任务已进入完成阶段，请作为新要求发送。", ModelMessage: "Start a new run with this instruction.", HTTPStatus: 409},
	"RUN_CANCELED":                   {Code: "RUN_CANCELED", Category: ErrorCanceled, SafeMessage: "任务已取消。", ModelMessage: "The run was canceled; do not retry this action.", HTTPStatus: 409},
	"RUN_ALREADY_CANCELED":           {Code: "RUN_ALREADY_CANCELED", Category: ErrorCanceled, SafeMessage: "任务已取消。", ModelMessage: "The run was already canceled; do not attempt to finish it.", HTTPStatus: 409},
	"RUN_FAILED":                     {Code: "RUN_FAILED", Category: ErrorTerminal, SafeMessage: "任务未能完成。", ModelMessage: "Stop the run and inspect the internal trace.", HTTPStatus: 500},
	"LOCK_TIMEOUT":                   {Code: "LOCK_TIMEOUT", Category: ErrorTransient, SafeMessage: "等待项目执行资源超时，请稍后重试。", ModelMessage: "Retry the run after the project execution slot becomes available.", Retryable: true, HTTPStatus: 503},
	"RESOURCE_INVALID":               {Code: "RESOURCE_INVALID", Category: ErrorAgentRepairable, SafeMessage: "PPT 资源地址不合法。", ModelMessage: `resource must be a structured object with kind deck, outline, design, or slide. Slide resources require a stable runtime-issued slide_id and part spec|html. Never use an ordinal, path, "current", or a model-generated ID.`, HTTPStatus: 422},
	"CAPABILITY_DENIED":              {Code: "CAPABILITY_DENIED", Category: ErrorTerminal, SafeMessage: "当前运行配置拒绝了已开放的操作，任务已停止。", ModelMessage: "Stop the run and inspect the Runtime tool-policy trace; retrying the same call cannot change authorization.", HTTPStatus: 500},
	"RESOURCE_NOT_DISCLOSED":         {Code: "RESOURCE_NOT_DISCLOSED", Category: ErrorAgentRepairable, SafeMessage: "当前步骤不能使用该资源。", ModelMessage: "Use only resources and tools disclosed for this turn.", HTTPStatus: 422},
	"TARGET_OUT_OF_SCOPE":            {Code: "TARGET_OUT_OF_SCOPE", Category: ErrorAgentRepairable, SafeMessage: "目标超出当前任务范围。", ModelMessage: "Keep the operation inside the current RunCommand scope.", HTTPStatus: 409},
	"RESOURCE_NOT_FOUND":             {Code: "RESOURCE_NOT_FOUND", Category: ErrorAgentRepairable, SafeMessage: "未找到所需 PPT 内容。", ModelMessage: "Read the outline and choose an existing Resource.", HTTPStatus: 404},
	"CONTENT_TOO_LARGE":              {Code: "CONTENT_TOO_LARGE", Category: ErrorAgentRepairable, SafeMessage: "PPT 内容超过处理上限。", ModelMessage: "Reduce the resource size before retrying.", HTTPStatus: 422},
	"CONTENT_INVALID":                {Code: "CONTENT_INVALID", Category: ErrorAgentRepairable, SafeMessage: "PPT 内容未通过格式检查。", ModelMessage: "Correct the reported JSON Pointer or HTML checks.", HTTPStatus: 422},
	"PATCH_INVALID":                  {Code: "PATCH_INVALID", Category: ErrorAgentRepairable, SafeMessage: "PPT 修改指令格式不正确。", ModelMessage: "Correct the RFC 6902 add, remove, or replace operation and retry.", HTTPStatus: 422},
	"PATCH_PATH_DENIED":              {Code: "PATCH_PATH_DENIED", Category: ErrorAgentRepairable, SafeMessage: "PPT 修改位置不允许写入。", ModelMessage: "Choose a writable JSON Pointer disclosed by the mutate_ppt schema.", HTTPStatus: 422},
	"EDIT_ANCHOR_NOT_FOUND":          {Code: "EDIT_ANCHOR_NOT_FOUND", Category: ErrorAgentRepairable, SafeMessage: "未找到要替换的内容。", ModelMessage: "Call read_ppt and use an exact current anchor.", HTTPStatus: 422},
	"EDIT_ANCHOR_AMBIGUOUS":          {Code: "EDIT_ANCHOR_AMBIGUOUS", Category: ErrorAgentRepairable, SafeMessage: "要替换的内容不唯一。", ModelMessage: "Call read_ppt and choose a longer unique anchor.", HTTPStatus: 422},
	"REVISION_CONFLICT":              {Code: "REVISION_CONFLICT", Category: ErrorConflict, SafeMessage: "PPT 已被其他操作更新，请重新读取后再试。", ModelMessage: "Read the current Resource revision before making another edit.", HTTPStatus: 409},
	"RUN_REVISION_CONFLICT":          {Code: "RUN_REVISION_CONFLICT", Category: ErrorConflict, SafeMessage: "PPT 已被其他操作更新，请重新读取后再试。", ModelMessage: "Read the current Resource revision before making another edit.", HTTPStatus: 409},
	"RENDER_FAILED":                  {Code: "RENDER_FAILED", Category: ErrorAgentRepairable, SafeMessage: "页面视觉检查未通过。", ModelMessage: "Fix the reported HTML or render diagnostics and render again.", HTTPStatus: 422},
	"TARGET_ALREADY_EXISTS":          {Code: "TARGET_ALREADY_EXISTS", Category: ErrorAgentRepairable, SafeMessage: "目标内容已经存在。", ModelMessage: "Read the current Resource and edit it instead of creating it again.", HTTPStatus: 409},
	"RUN_SESSION_REQUIRED":           {Code: "RUN_SESSION_REQUIRED", Category: ErrorAgentRepairable, SafeMessage: "当前操作需要活跃的写入会话。", ModelMessage: "Perform the write inside the active run session.", HTTPStatus: 422},
	"RUN_SESSION_MISSING":            {Code: "RUN_SESSION_MISSING", Category: ErrorAgentRepairable, SafeMessage: "当前操作需要活跃的写入会话。", ModelMessage: "Perform the write inside the active run session.", HTTPStatus: 422},
	"INVALID_CONTROL_CALL":           {Code: "INVALID_CONTROL_CALL", Category: ErrorAgentRepairable, SafeMessage: "Agent 控制动作不适用于当前阶段。", ModelMessage: "Choose a runtime control action disclosed for the current phase.", HTTPStatus: 422},
	"SCOPE_EXPANSION_REQUIRED":       {Code: "SCOPE_EXPANSION_REQUIRED", Category: ErrorAgentRepairable, SafeMessage: "任务范围需要先更新计划。", ModelMessage: "Update the plan in the same ReAct loop before expanding the work scope.", HTTPStatus: 422},
	"DEPENDENCY_FAILED":              {Code: "DEPENDENCY_FAILED", Category: ErrorAgentRepairable, SafeMessage: "前序操作失败，后续操作未执行。", ModelMessage: "Repair the first failed call before retrying dependent calls.", HTTPStatus: 422},
	"COMPLETION_GATE_BLOCKED":        {Code: "COMPLETION_GATE_BLOCKED", Category: ErrorAgentRepairable, SafeMessage: "任务尚未满足完成条件。", ModelMessage: "Perform the required next actions reported by the completion gate.", HTTPStatus: 422},
	"COMPLETION_GATE_BLOCK":          {Code: "COMPLETION_GATE_BLOCK", Category: ErrorAgentRepairable, SafeMessage: "任务尚未满足完成条件。", ModelMessage: "Perform the required next actions reported by the completion gate.", HTTPStatus: 422},
	"COMPLETION_REVIEW_BLOCK":        {Code: "COMPLETION_REVIEW_BLOCK", Category: ErrorAgentRepairable, SafeMessage: "任务尚未通过最终审查。", ModelMessage: "Perform the required next actions reported by the completion reviewer.", HTTPStatus: 422},
	"CONSECUTIVE_TOOL_ERRORS":        {Code: "CONSECUTIVE_TOOL_ERRORS", Category: ErrorTerminal, SafeMessage: "连续修正未成功，任务已停止。", ModelMessage: "Stop after the bounded repair budget is exhausted.", HTTPStatus: 500},
	"COMPLETION_REJECTED_REPEATEDLY": {Code: "COMPLETION_REJECTED_REPEATEDLY", Category: ErrorTerminal, SafeMessage: "多次检查仍未满足完成条件，任务已停止。", ModelMessage: "Stop after repeated identical completion-gate rejections.", HTTPStatus: 500},
	"COMPLETION_BLOCK_REPEAT":        {Code: "COMPLETION_BLOCK_REPEAT", Category: ErrorTerminal, SafeMessage: "多次检查仍未满足完成条件，任务已停止。", ModelMessage: "Stop after repeated identical completion blocks.", HTTPStatus: 500},
	"CONTEXT_BUDGET_EXCEEDED":        {Code: "CONTEXT_BUDGET_EXCEEDED", Category: ErrorUserActionRequired, SafeMessage: "上下文超过处理上限。", ModelMessage: "Reduce the selected context before retrying.", HTTPStatus: 413},
	"PROVIDER_BAD_REQUEST":           {Code: "PROVIDER_BAD_REQUEST", Category: ErrorTerminal, SafeMessage: "模型服务拒绝了本次请求，请检查模型配置或请求格式。", ModelMessage: "Stop the run and inspect the provider bad-request diagnostic in trace/logs.", HTTPStatus: 502},
	"PROVIDER_UNAVAILABLE":           {Code: "PROVIDER_UNAVAILABLE", Category: ErrorTransient, SafeMessage: "模型服务暂时不可用，正在尝试恢复。", ModelMessage: "Retry the provider request with bounded backoff.", Retryable: true, HTTPStatus: 503},
	"POLISH_OUTPUT_INVALID":          {Code: "POLISH_OUTPUT_INVALID", Category: ErrorTransient, SafeMessage: "润色结果暂时不可用，请重试。", ModelMessage: "Retry prompt polishing without changing the original instruction.", Retryable: true, HTTPStatus: 503},
	"PROJECT_EMPTY":                  {Code: "PROJECT_EMPTY", Category: ErrorUserActionRequired, SafeMessage: "当前项目暂无内容，请先创建页面。", ModelMessage: "Add project content before generating a briefing.", HTTPStatus: 409},
	"BRIEFING_ACTIVE":                {Code: "BRIEFING_ACTIVE", Category: ErrorConflict, SafeMessage: "项目有其他操作正在进行，请稍后重试。", ModelMessage: "Wait for the active project operation to finish.", HTTPStatus: 409},
	"COMPACT_ACTIVE":                 {Code: "COMPACT_ACTIVE", Category: ErrorConflict, SafeMessage: "项目有其他操作正在进行，请稍后重试。", ModelMessage: "Wait for the active project operation to finish.", HTTPStatus: 409},
	"BRIEFING_OUTPUT_INVALID":        {Code: "BRIEFING_OUTPUT_INVALID", Category: ErrorTransient, SafeMessage: "简报结果暂时不可用，请重试。", ModelMessage: "Retry briefing generation.", Retryable: true, HTTPStatus: 503},
	"RENDER_WORKER_UNAVAILABLE":      {Code: "RENDER_WORKER_UNAVAILABLE", Category: ErrorTransient, SafeMessage: "页面渲染服务暂时不可用。", ModelMessage: "Restart the render worker and retry once.", Retryable: true, HTTPStatus: 503},
	"COMMIT_FAILED":                  {Code: "COMMIT_FAILED", Category: ErrorTerminal, SafeMessage: "修改未能安全保存，请重新发起任务。", ModelMessage: "Do not retry an unconfirmed commit without querying its idempotency record.", HTTPStatus: 500},
	"RUNTIME_BUDGET_EXCEEDED":        {Code: "RUNTIME_BUDGET_EXCEEDED", Category: ErrorTerminal, SafeMessage: "运行达到轮次或时长上限，未完成的修改未保存。", ModelMessage: "Stop the run; the uncommitted RunSession overlay is discarded.", HTTPStatus: 500},
	"AGENT_FAILED":                   {Code: "AGENT_FAILED", Category: ErrorTerminal, SafeMessage: "Agent 暂时无法继续，请稍后重试。", ModelMessage: "Stop the run and preserve the internal cause in trace only.", HTTPStatus: 500},
	"INTERNAL":                       {Code: "INTERNAL", Category: ErrorTerminal, SafeMessage: "服务暂时无法完成请求。", ModelMessage: "Stop and inspect the internal trace.", HTTPStatus: 500},
	"FINISH_MESSAGE_EMPTY":           {Code: "FINISH_MESSAGE_EMPTY", Category: ErrorAgentRepairable, SafeMessage: "最终回复不能为空。", ModelMessage: "Call finish with a non-empty message.", HTTPStatus: 422},
	"TOOLS_STILL_RUNNING":            {Code: "TOOLS_STILL_RUNNING", Category: ErrorAgentRepairable, SafeMessage: "仍有工具正在执行。", ModelMessage: "Wait for running tools to complete before finish.", HTTPStatus: 422},
	"RUN_FATAL_EXIST":                {Code: "RUN_FATAL_EXIST", Category: ErrorTerminal, SafeMessage: "任务存在严重错误，无法完成。", ModelMessage: "Stop and inspect the fatal runtime issue.", HTTPStatus: 500},
	"PLAN_NOT_COMPLETE":              {Code: "PLAN_NOT_COMPLETE", Category: ErrorAgentRepairable, SafeMessage: "执行计划尚未完成。", ModelMessage: "Complete, repair, or update all plan steps before finish.", HTTPStatus: 422},
	"EVIDENCE_SCHEMA_MISSING":        {Code: "EVIDENCE_SCHEMA_MISSING", Category: ErrorAgentRepairable, SafeMessage: "结构化内容缺少校验证据。", ModelMessage: "Rewrite or repair the changed JSON resource so schema evidence is recorded.", HTTPStatus: 422},
	"EVIDENCE_HTML_MISSING":          {Code: "EVIDENCE_HTML_MISSING", Category: ErrorAgentRepairable, SafeMessage: "HTML 内容缺少校验或渲染证据。", ModelMessage: "Repair the changed HTML and render the affected slide.", HTTPStatus: 422},
	"ASYNC_SPEC_HTML":                {Code: "ASYNC_SPEC_HTML", Category: ErrorAgentRepairable, SafeMessage: "设计稿与 HTML 尚未同步。", ModelMessage: "Update the affected HTML to reflect the changed spec or design.", HTTPStatus: 422},
	"ASYNC_DECK_SLIDE":               {Code: "ASYNC_DECK_SLIDE", Category: ErrorAgentRepairable, SafeMessage: "目录与页面规格尚未同步。", ModelMessage: "Repair outline and slide spec references before finish.", HTTPStatus: 422},
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
