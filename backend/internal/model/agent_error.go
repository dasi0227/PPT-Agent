package model

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type ErrorCategory string

const (
	ErrorTransient   ErrorCategory = "transient"
	ErrorRepairable  ErrorCategory = "repairable"
	ErrorInterrupted ErrorCategory = "interrupted"
	ErrorConflict    ErrorCategory = "conflict"
	ErrorCanceled    ErrorCategory = "canceled"
	ErrorTerminal    ErrorCategory = "terminal"
)

type ErrorDefinition struct {
	Code         string
	Category     ErrorCategory
	SafeMessage  string
	ModelMessage string
	Retryable    bool
	HTTPStatus   int
}

var errorDefinitions = map[string]ErrorDefinition{
	CodeRunCanceled:                  {Code: CodeRunCanceled, Category: ErrorCanceled, SafeMessage: "任务已取消。", ModelMessage: "The run was canceled; do not retry this action.", HTTPStatus: 409},
	CodeDependencyFailed:             {Code: CodeDependencyFailed, Category: ErrorRepairable, SafeMessage: "前序操作未生效，后续操作未执行。", ModelMessage: "Inspect the required predecessor replies, respect refusals and resolve failed prerequisites within authorization before regenerating the dependent call.", HTTPStatus: 422},
	CodeResourceNotDisclosed:         {Code: CodeResourceNotDisclosed, Category: ErrorRepairable, SafeMessage: "当前步骤不能使用该工具或资源。", ModelMessage: "Use only tools and resources disclosed for this turn.", HTTPStatus: 422},
	CodeToolArgumentInvalid:          {Code: CodeToolArgumentInvalid, Category: ErrorRepairable, SafeMessage: "工具参数格式不正确，正在修正。", ModelMessage: "Correct the reported field to match the disclosed input schema, then call the tool again. Pass arrays and objects as JSON values, not quoted JSON strings. Do not repeat unchanged arguments.", HTTPStatus: 422},
	CodeToolCallInvalid:              {Code: CodeToolCallInvalid, Category: ErrorRepairable, SafeMessage: "工具调用方式不合法，正在修正。", ModelMessage: "Correct the tool choice, call count, execution order or isolated-call requirement, and omit ordinary text when the entry requires a tool-only response. No rejected call was executed.", HTTPStatus: 422},
	CodeIdempotencyKeyReused:         {Code: CodeIdempotencyKeyReused, Category: ErrorConflict, SafeMessage: "该请求标识已用于其他内容，请重新发送。", ModelMessage: "Use a new call or request identifier for a different tool or different arguments. Never change the arguments of an already recorded call.", HTTPStatus: 409},
	CodeTargetOutOfScope:             {Code: CodeTargetOutOfScope, Category: ErrorRepairable, SafeMessage: "目标超出当前任务范围。", ModelMessage: "Use an authorized slide_id, or call request_privilege with a reason and the required scope additions when disclosed. Wait for approval before acting outside the current scope.", HTTPStatus: 409},
	CodePersistenceFailed:            {Code: CodePersistenceFailed, Category: ErrorTerminal, SafeMessage: "修改未能安全保存，请重新发起任务。", ModelMessage: "Stop the run. Verify the saved files and operation receipt before another write; do not repeat an operation whose persistence outcome is uncertain.", HTTPStatus: 500},
	CodeInternal:                     {Code: CodeInternal, Category: ErrorTerminal, SafeMessage: "服务暂时无法完成请求。", ModelMessage: "Stop the run and inspect the internal trace. Model calls cannot repair missing services, inconsistent Runtime configuration or broken execution state.", HTTPStatus: 500},
	"RUN_REVISION_CONFLICT":          {Code: "RUN_REVISION_CONFLICT", Category: ErrorConflict, SafeMessage: "任务范围已更新，请重试。", ModelMessage: "Read the current RunScope before retrying the control input.", HTTPStatus: 409},
	"BAD_REQUEST":                    {Code: "BAD_REQUEST", Category: ErrorInterrupted, SafeMessage: "请求内容不合法。", ModelMessage: "Correct the request and try again.", HTTPStatus: 400},
	"NOT_FOUND":                      {Code: "NOT_FOUND", Category: ErrorInterrupted, SafeMessage: "未找到请求的内容。", ModelMessage: "Choose a resource that exists in the current project.", HTTPStatus: 404},
	"CONFLICT":                       {Code: "CONFLICT", Category: ErrorConflict, SafeMessage: "请求与当前状态冲突，请刷新后重试。", ModelMessage: "Refresh authoritative state before the next action.", HTTPStatus: 409},
	"RUN_NOT_FOUND":                  {Code: "RUN_NOT_FOUND", Category: ErrorInterrupted, SafeMessage: "未找到当前任务。", ModelMessage: "The requested run does not exist.", HTTPStatus: 404},
	"SLIDE_NOT_FOUND":                {Code: "SLIDE_NOT_FOUND", Category: ErrorInterrupted, SafeMessage: "未找到指定页面。", ModelMessage: "Choose a slide_id that belongs to the current project.", HTTPStatus: 400},
	"INVALID_SCOPE":                  {Code: "INVALID_SCOPE", Category: ErrorInterrupted, SafeMessage: "任务范围不合法。", ModelMessage: "Correct scope.selection using stable slide or section IDs.", HTTPStatus: 422},
	"RUN_SCOPE_UNSUPPORTED":          {Code: "RUN_SCOPE_UNSUPPORTED", Category: ErrorInterrupted, SafeMessage: "当前任务范围暂不支持。", ModelMessage: "Choose a supported PPT scope.", HTTPStatus: 422},
	"MODEL_PROFILE_NOT_FOUND":        {Code: "MODEL_PROFILE_NOT_FOUND", Category: ErrorInterrupted, SafeMessage: "所选模型已不可用，请重新选择。", ModelMessage: "Choose an exact model profile returned by the server.", HTTPStatus: 422},
	"ATTACHMENT_TYPE_UNSUPPORTED":    {Code: "ATTACHMENT_TYPE_UNSUPPORTED", Category: ErrorInterrupted, SafeMessage: "仅支持 PNG 和 JPG 图片。", ModelMessage: "Use a PNG or JPEG static image.", HTTPStatus: 422},
	"ATTACHMENT_TOO_LARGE":           {Code: "ATTACHMENT_TOO_LARGE", Category: ErrorInterrupted, SafeMessage: "图片超过 10 MiB，请压缩后重试。", ModelMessage: "Choose an image smaller than 10 MiB.", HTTPStatus: 422},
	"ATTACHMENT_DIMENSIONS_EXCEEDED": {Code: "ATTACHMENT_DIMENSIONS_EXCEEDED", Category: ErrorInterrupted, SafeMessage: "图片分辨率过高，请缩小后重试。", ModelMessage: "Choose an image with at most 40 megapixels.", HTTPStatus: 422},
	"ATTACHMENT_INVALID":             {Code: "ATTACHMENT_INVALID", Category: ErrorInterrupted, SafeMessage: "无法读取这张图片，请更换文件。", ModelMessage: "Choose a valid static image with matching content and MIME type.", HTTPStatus: 422},
	"ATTACHMENT_NOT_FOUND":           {Code: "ATTACHMENT_NOT_FOUND", Category: ErrorInterrupted, SafeMessage: "附件已不可用，请重新选择。", ModelMessage: "Choose an attachment that belongs to the current project.", HTTPStatus: 404},
	"ATTACHMENT_LIMIT_EXCEEDED":      {Code: "ATTACHMENT_LIMIT_EXCEEDED", Category: ErrorInterrupted, SafeMessage: "每条消息最多添加 8 张图片。", ModelMessage: "Limit the message to eight image attachments.", HTTPStatus: 422},
	"DOM_SELECTION_INVALID":          {Code: "DOM_SELECTION_INVALID", Category: ErrorInterrupted, SafeMessage: "DOM 标记数据无效，请重新选择。", ModelMessage: "Choose DOM elements from the current project preview.", HTTPStatus: 422},
	"DOM_SELECTION_LIMIT":            {Code: "DOM_SELECTION_LIMIT", Category: ErrorInterrupted, SafeMessage: "选择内容过多，请缩小范围。", ModelMessage: "Limit the message to eight DOM selections and fifty targets per selection.", HTTPStatus: 422},
	"DOM_SELECTION_TOO_LARGE":        {Code: "DOM_SELECTION_TOO_LARGE", Category: ErrorInterrupted, SafeMessage: "选择内容过大，请缩小范围。", ModelMessage: "Reduce the selected DOM snapshot size.", HTTPStatus: 422},
	"DOM_SELECTION_COMMENT_TOO_LONG": {Code: "DOM_SELECTION_COMMENT_TOO_LONG", Category: ErrorInterrupted, SafeMessage: "标记注释最多 500 个字符。", ModelMessage: "Shorten the DOM selection comment to 500 characters.", HTTPStatus: 422},
	"REFERENCE_ORDER_INVALID":        {Code: "REFERENCE_ORDER_INVALID", Category: ErrorInterrupted, SafeMessage: "消息引用顺序无效，请重试。", ModelMessage: "Reference every image and DOM selection exactly once.", HTTPStatus: 422},
	"SKILL_SELECTION_INVALID":        {Code: "SKILL_SELECTION_INVALID", Category: ErrorInterrupted, SafeMessage: "技能选择不合法，请重新选择。", ModelMessage: "Choose at most three unique skills returned by the server.", HTTPStatus: 422},
	"SKILL_NOT_FOUND":                {Code: "SKILL_NOT_FOUND", Category: ErrorInterrupted, SafeMessage: "所选技能已不可用，请重新选择。", ModelMessage: "Choose an exact skill id returned by the server.", HTTPStatus: 422},
	"SKILL_DISABLED":                 {Code: "SKILL_DISABLED", Category: ErrorInterrupted, SafeMessage: "所选技能已禁用，请重新选择。", ModelMessage: "Choose an enabled skill returned by the repository catalog.", HTTPStatus: 409},
	"COMPONENT_SELECTION_INVALID":    {Code: "COMPONENT_SELECTION_INVALID", Category: ErrorInterrupted, SafeMessage: "组件引用不合法，请重新选择。", ModelMessage: "Choose at most eight component names returned by the server.", HTTPStatus: 400},
	"COMPONENT_NOT_FOUND":            {Code: "COMPONENT_NOT_FOUND", Category: ErrorInterrupted, SafeMessage: "引用的组件已不存在或已改名，请重新选择。", ModelMessage: "Choose an exact enabled component name returned by the server.", HTTPStatus: 404},
	"COMPONENT_NAME_AMBIGUOUS":       {Code: "COMPONENT_NAME_AMBIGUOUS", Category: ErrorConflict, SafeMessage: "存在重名组件，请先在组件仓库中重命名。", ModelMessage: "Rename duplicate enabled components before retrying.", HTTPStatus: 409},
	"COMPONENT_DISABLED":             {Code: "COMPONENT_DISABLED", Category: ErrorConflict, SafeMessage: "引用的组件已被禁用，请重新选择。", ModelMessage: "Enable the component or choose another enabled component.", HTTPStatus: 409},
	"PAGE_SELECTION_INVALID":         {Code: "PAGE_SELECTION_INVALID", Category: ErrorInterrupted, SafeMessage: "页面引用不合法，请重新选择。", ModelMessage: "Choose at most eight stable slide_ids from the current outline.", HTTPStatus: 400},
	"RUN_WAITING_FOR_ANSWER":         {Code: "RUN_WAITING_FOR_ANSWER", Category: ErrorConflict, SafeMessage: "当前任务正在等待问题回答，请先回答问题。", ModelMessage: "Answer the pending ask_user question instead of steering.", HTTPStatus: 409},
	"RUN_CANCELING":                  {Code: "RUN_CANCELING", Category: ErrorCanceled, SafeMessage: "当前任务正在取消，无法再追加要求。", ModelMessage: "Wait for the canceled terminal state.", HTTPStatus: 409},
	"RUN_NOT_STEERABLE":              {Code: "RUN_NOT_STEERABLE", Category: ErrorConflict, SafeMessage: "当前任务已进入完成阶段，请作为新要求发送。", ModelMessage: "Start a new run with this instruction.", HTTPStatus: 409},
	"RUN_ALREADY_CANCELED":           {Code: "RUN_ALREADY_CANCELED", Category: ErrorCanceled, SafeMessage: "任务已取消。", ModelMessage: "The run was already canceled; do not attempt to finish it.", HTTPStatus: 409},
	"RUN_FAILED":                     {Code: "RUN_FAILED", Category: ErrorTerminal, SafeMessage: "任务未能完成。", ModelMessage: "Stop the run and inspect the internal trace.", HTTPStatus: 500},
	"LOCK_TIMEOUT":                   {Code: "LOCK_TIMEOUT", Category: ErrorTransient, SafeMessage: "等待项目执行资源超时，请稍后重试。", ModelMessage: "Retry the run after the project execution slot becomes available.", Retryable: true, HTTPStatus: 503},
	"RESOURCE_INVALID":               {Code: "RESOURCE_INVALID", Category: ErrorRepairable, SafeMessage: "PPT 资源地址不合法。", ModelMessage: `read_resource requires type: "manifest" | "design" | "outline" | "spec" | "html". Only spec/html require slide_id; global resources forbid it. Use an existing Runtime-issued slide_id, never an ordinal or invented ID.`, HTTPStatus: 422},
	"RESOURCE_NOT_FOUND":             {Code: "RESOURCE_NOT_FOUND", Category: ErrorRepairable, SafeMessage: "未找到所需内容。", ModelMessage: "Use an existing resource or repository ID from the current context; follow the tool-specific creation or refresh instructions. Do not repeat an unchanged read of missing content.", HTTPStatus: 404},
	"OUTLINE_NOT_INITIALIZED":        {Code: "OUTLINE_NOT_INITIALIZED", Category: ErrorRepairable, SafeMessage: "目录结构尚未初始化。", ModelMessage: "There is no saved outline yet. Use edit_outline(init: <complete JSON object>) when it is disclosed and creation is needed; otherwise continue without an outline. Do not retry reading it or invent slide IDs.", HTTPStatus: 404},
	"RESOURCE_CONTENT_INVALID":       {Code: "RESOURCE_CONTENT_INVALID", Category: ErrorInterrupted, SafeMessage: "已保存的资源内容损坏，需要修复后再读取。", ModelMessage: "The saved resource exists but cannot be parsed or validated. Report the damaged resource; do not treat it as missing, reinitialize it, or repeatedly read unchanged content.", HTTPStatus: 422},
	"READ_FAILED":                    {Code: "READ_FAILED", Category: ErrorTerminal, SafeMessage: "资源读取失败，请检查存储状态。", ModelMessage: "Resource I/O failed. Inspect storage or permissions; do not treat unreadable content as absent or recreate it.", HTTPStatus: 500},
	"WRITE_FAILED":                   {Code: "WRITE_FAILED", Category: ErrorTerminal, SafeMessage: "资源写入失败，请检查存储状态。", ModelMessage: "Resource I/O failed while writing. Inspect storage and the saved state before another write; do not claim this operation succeeded.", HTTPStatus: 500},
	"CONTENT_TOO_LARGE":              {Code: "CONTENT_TOO_LARGE", Category: ErrorRepairable, SafeMessage: "PPT 内容超过处理上限。", ModelMessage: "Reduce the resource size before retrying.", HTTPStatus: 422},
	"CONTENT_INVALID":                {Code: "CONTENT_INVALID", Category: ErrorRepairable, SafeMessage: "PPT 内容未通过格式检查。", ModelMessage: "Correct the reported JSON Pointer or HTML checks.", HTTPStatus: 422},
	"EDIT_ANCHOR_NOT_FOUND":          {Code: "EDIT_ANCHOR_NOT_FOUND", Category: ErrorRepairable, SafeMessage: "未找到要替换的内容。", ModelMessage: "Call read_resource and use an exact current anchor.", HTTPStatus: 422},
	"EDIT_ANCHOR_AMBIGUOUS":          {Code: "EDIT_ANCHOR_AMBIGUOUS", Category: ErrorRepairable, SafeMessage: "要替换的内容不唯一。", ModelMessage: "Call read_resource and choose a longer unique anchor.", HTTPStatus: 422},
	"CONTENT_CONFLICT":               {Code: "CONTENT_CONFLICT", Category: ErrorConflict, SafeMessage: "PPT 已被其他操作更新，请重新读取后再试。", ModelMessage: "Read the current resource with read_resource, then recompute the edit using that content. Runtime manages version checks.", HTTPStatus: 409},
	"RUN_CONTENT_CONFLICT":           {Code: "RUN_CONTENT_CONFLICT", Category: ErrorConflict, SafeMessage: "PPT 已被其他操作更新，请重新读取后再试。", ModelMessage: "Read the current resource with read_resource, then recompute the edit using that content. Runtime manages version checks.", HTTPStatus: 409},
	"RENDER_FAILED":                  {Code: "RENDER_FAILED", Category: ErrorRepairable, SafeMessage: "页面视觉检查未通过。", ModelMessage: "Fix the reported HTML or render diagnostics and render again.", HTTPStatus: 422},
	"TARGET_ALREADY_EXISTS":          {Code: "TARGET_ALREADY_EXISTS", Category: ErrorRepairable, SafeMessage: "目标内容已经存在。", ModelMessage: "Read the current Resource and edit it instead of creating it again.", HTTPStatus: 409},
	"PLAN_INVALID":                   {Code: "PLAN_INVALID", Category: ErrorRepairable, SafeMessage: "计划更新不符合当前规则，正在修正。", ModelMessage: "Correct the reported plan issue using the current plan and disclosed tool parameters. Use existing step IDs and valid statuses; do not replace an active plan or mark unfinished work complete.", HTTPStatus: 422},
	"SCOPE_EXPANSION_REQUIRED":       {Code: "SCOPE_EXPANSION_REQUIRED", Category: ErrorRepairable, SafeMessage: "需要申请扩大任务范围。", ModelMessage: "Call request_privilege with a reason and the required scope additions when disclosed; wait for approval. Updating plan progress does not grant permission.", HTTPStatus: 422},
	"COMMAND_PARSE_INVALID":          {Code: "COMMAND_PARSE_INVALID", Category: ErrorRepairable, SafeMessage: "命令语法不正确。", ModelMessage: "Correct the reported command syntax and quoting, then submit the corrected command.", HTTPStatus: 422},
	"COMMAND_SYNTAX_DENIED":          {Code: "COMMAND_SYNTAX_DENIED", Category: ErrorRepairable, SafeMessage: "命令使用了不支持的语法。", ModelMessage: "Use only syntax supported by run_command; simplify the command as reported. Submit an approval-bound edit as a single command.", HTTPStatus: 422},
	"COMMAND_NOT_ALLOWED":            {Code: "COMMAND_NOT_ALLOWED", Category: ErrorRepairable, SafeMessage: "当前不支持执行该命令。", ModelMessage: "Choose a supported command from the disclosed run_command schema; do not bypass the command policy.", HTTPStatus: 422},
	"COMMAND_NOT_AVAILABLE":          {Code: "COMMAND_NOT_AVAILABLE", Category: ErrorRepairable, SafeMessage: "该命令当前不可用。", ModelMessage: "Choose an available supported command or use the relevant resource tool; do not repeat the unavailable command.", HTTPStatus: 422},
	"COMMAND_FLAG_DENIED":            {Code: "COMMAND_FLAG_DENIED", Category: ErrorRepairable, SafeMessage: "命令参数不受支持。", ModelMessage: "Remove or correct the reported unsupported flag while keeping the operation within the command policy.", HTTPStatus: 422},
	"COMMAND_PATH_OUTSIDE_PROJECT":   {Code: "COMMAND_PATH_OUTSIDE_PROJECT", Category: ErrorRepairable, SafeMessage: "命令路径超出项目范围。", ModelMessage: "Use a path inside the current project. Do not bypass the boundary through traversal, links or another command.", HTTPStatus: 422},
	"COMMAND_PATH_INVALID":           {Code: "COMMAND_PATH_INVALID", Category: ErrorRepairable, SafeMessage: "命令指定的路径无效。", ModelMessage: "Inspect the allowed project directory and use an existing valid project-relative path.", HTTPStatus: 422},
	"COMMAND_SENSITIVE_READ_DENIED":  {Code: "COMMAND_SENSITIVE_READ_DENIED", Category: ErrorRepairable, SafeMessage: "该命令无权读取敏感内容。", ModelMessage: "Respect the read restriction. Continue using authorized content; do not seek an alternate route to the denied data.", HTTPStatus: 403},
	"COMMAND_EXIT_NONZERO":           {Code: "COMMAND_EXIT_NONZERO", Category: ErrorRepairable, SafeMessage: "命令返回了非零退出状态。", ModelMessage: "Inspect exit_code, stdout and stderr. Interpret a no-match result correctly; otherwise correct the command or its input before retrying.", HTTPStatus: 422},
	"COMMAND_TIMEOUT":                {Code: "COMMAND_TIMEOUT", Category: ErrorRepairable, SafeMessage: "命令执行超时。", ModelMessage: "Reduce the command scope or choose a narrower query before retrying.", HTTPStatus: 422},
	"COMMAND_OUTPUT_LIMIT":           {Code: "COMMAND_OUTPUT_LIMIT", Category: ErrorRepairable, SafeMessage: "命令输出超过上限。", ModelMessage: "Use narrower paths, filters or bounded output. Do not treat truncated output as complete.", HTTPStatus: 422},
	"COMMAND_EXEC_FAILED":            {Code: "COMMAND_EXEC_FAILED", Category: ErrorRepairable, SafeMessage: "命令未能执行成功。", ModelMessage: "Inspect the reported execution failure and choose a corrected supported command. If the environment is unavailable, report the limitation.", HTTPStatus: 422},
	"COMPLETION_GATE_BLOCK":          {Code: "COMPLETION_GATE_BLOCK", Category: ErrorRepairable, SafeMessage: "任务尚未满足完成条件。", ModelMessage: "Perform the required next actions reported by the completion gate.", HTTPStatus: 422},
	"REVIEW_FAILED":                  {Code: "REVIEW_FAILED", Category: ErrorRepairable, SafeMessage: "未完成成果审查。", ModelMessage: "Artifact review could not produce a valid result. Inspect the error and address missing materials or service problems; do not treat this as an assessment or approval.", HTTPStatus: 422},
	"CONSECUTIVE_TOOL_ERRORS":        {Code: "CONSECUTIVE_TOOL_ERRORS", Category: ErrorTerminal, SafeMessage: "连续修正未成功，任务已停止。", ModelMessage: "Stop after the bounded repair budget is exhausted.", HTTPStatus: 500},
	"REPEATED_READ_NO_PROGRESS":      {Code: "REPEATED_READ_NO_PROGRESS", Category: ErrorTerminal, SafeMessage: "任务反复读取相同内容且未取得新进展，已停止以避免继续消耗。", ModelMessage: "Stop repeated reads of unchanged results. Preserve saved work; continue only after an explicit user action.", HTTPStatus: 500},
	"COMPLETION_BLOCK_REPEAT":        {Code: "COMPLETION_BLOCK_REPEAT", Category: ErrorTerminal, SafeMessage: "多次检查仍未满足完成条件，任务已停止。", ModelMessage: "Stop after repeated identical completion blocks.", HTTPStatus: 500},
	"CONTEXT_BUDGET_EXCEEDED":        {Code: "CONTEXT_BUDGET_EXCEEDED", Category: ErrorInterrupted, SafeMessage: "上下文超过处理上限。", ModelMessage: "Reduce the selected context before retrying.", HTTPStatus: 413},
	"PROVIDER_BAD_REQUEST":           {Code: "PROVIDER_BAD_REQUEST", Category: ErrorTerminal, SafeMessage: "模型服务拒绝了本次请求，请检查模型配置或请求格式。", ModelMessage: "Stop the run and inspect the provider bad-request diagnostic in trace/logs.", HTTPStatus: 502},
	"IMAGE_REFERENCE_UNAVAILABLE":    {Code: "IMAGE_REFERENCE_UNAVAILABLE", Category: ErrorTerminal, SafeMessage: "图片引用已失效或无法读取，请重新渲染页面或重新添加图片后再试。", ModelMessage: "A local image reference could not be read before contacting the provider. Refresh the render or attachment reference.", HTTPStatus: 422},
	"PROVIDER_UNAVAILABLE":           {Code: "PROVIDER_UNAVAILABLE", Category: ErrorTransient, SafeMessage: "模型服务请求失败。", ModelMessage: "The provider request failed. Preserve saved work; continuation requires an explicit user action.", Retryable: true, HTTPStatus: 503},
	"POLISH_OUTPUT_INVALID":          {Code: "POLISH_OUTPUT_INVALID", Category: ErrorTransient, SafeMessage: "润色结果暂时不可用，请重试。", ModelMessage: "Retry prompt polishing without changing the original instruction.", Retryable: true, HTTPStatus: 503},
	"PROJECT_EMPTY":                  {Code: "PROJECT_EMPTY", Category: ErrorInterrupted, SafeMessage: "当前项目暂无内容，请先创建页面。", ModelMessage: "Add project content before generating a briefing.", HTTPStatus: 409},
	"BRIEFING_ACTIVE":                {Code: "BRIEFING_ACTIVE", Category: ErrorConflict, SafeMessage: "项目有其他操作正在进行，请稍后重试。", ModelMessage: "Wait for the active project operation to finish.", HTTPStatus: 409},
	"COMPACT_ACTIVE":                 {Code: "COMPACT_ACTIVE", Category: ErrorConflict, SafeMessage: "项目有其他操作正在进行，请稍后重试。", ModelMessage: "Wait for the active project operation to finish.", HTTPStatus: 409},
	"COMPACT_BELOW_THRESHOLD":        {Code: "COMPACT_BELOW_THRESHOLD", Category: ErrorInterrupted, SafeMessage: "可压缩历史不足 12k Token，继续对话后再试。", ModelMessage: "Wait until the compactable transcript reaches 12000 tokens.", HTTPStatus: 409},
	"BRIEFING_OUTPUT_INVALID":        {Code: "BRIEFING_OUTPUT_INVALID", Category: ErrorTransient, SafeMessage: "简报结果暂时不可用，请重试。", ModelMessage: "Retry briefing generation.", Retryable: true, HTTPStatus: 503},
	"RENDER_WORKER_UNAVAILABLE":      {Code: "RENDER_WORKER_UNAVAILABLE", Category: ErrorTransient, SafeMessage: "页面渲染服务暂时不可用。", ModelMessage: "Restart the render worker and retry once.", Retryable: true, HTTPStatus: 503},
	"GIT_COMMIT_FAILED":              {Code: "GIT_COMMIT_FAILED", Category: ErrorTerminal, SafeMessage: "项目版本提交未能完成，请检查提交记录。", ModelMessage: "Inspect local Git history and report the failure. Do not issue a new git_commit call to repeat an unconfirmed commit.", HTTPStatus: 500},
	"RUNTIME_BUDGET_EXCEEDED":        {Code: "RUNTIME_BUDGET_EXCEEDED", Category: ErrorTerminal, SafeMessage: "任务已达到执行轮次或时长上限，未能完成全部要求。", ModelMessage: "Stop the run; the uncommitted RunSession overlay is discarded.", HTTPStatus: 500},
	"FINISH_MESSAGE_EMPTY":           {Code: "FINISH_MESSAGE_EMPTY", Category: ErrorRepairable, SafeMessage: "最终回复不能为空。", ModelMessage: "Call finish_task with a non-empty message.", HTTPStatus: 422},
	"FINISH_NOT_ALLOWED":             {Code: "FINISH_NOT_ALLOWED", Category: ErrorRepairable, SafeMessage: "当前阶段尚不能结束任务。", ModelMessage: "Use the control tools disclosed for the current phase; do not call finish_task before execution is allowed to complete.", HTTPStatus: 422},
	"CONTEXT_SOURCE_INVALID":         {Code: "CONTEXT_SOURCE_INVALID", Category: ErrorInterrupted, SafeMessage: "当前资源无法用于完成检查。", ModelMessage: "Inspect the reported source resource and its read error. Do not replace damaged saved content with an empty resource or claim completion; report storage problems if the resource cannot be safely read.", HTTPStatus: 422},
	"TOOLS_STILL_RUNNING":            {Code: "TOOLS_STILL_RUNNING", Category: ErrorRepairable, SafeMessage: "仍有工具正在执行。", ModelMessage: "Wait for running tools to complete before finish_task.", HTTPStatus: 422},
	"RUN_FATAL_EXIST":                {Code: "RUN_FATAL_EXIST", Category: ErrorTerminal, SafeMessage: "任务存在严重错误，无法完成。", ModelMessage: "Stop and inspect the fatal runtime issue.", HTTPStatus: 500},
	"PLAN_NOT_COMPLETE":              {Code: "PLAN_NOT_COMPLETE", Category: ErrorRepairable, SafeMessage: "执行计划尚未完成。", ModelMessage: "Complete the promised work and update permitted step statuses. Never mark attempted or failed work complete to bypass the check.", HTTPStatus: 422},
	"EVIDENCE_SCHEMA_MISSING":        {Code: "EVIDENCE_SCHEMA_MISSING", Category: ErrorRepairable, SafeMessage: "结构化内容缺少校验证据。", ModelMessage: "Runtime refreshes schema evidence from current saved content before completion. Do not rewrite unchanged content or invent evidence; repair source only when an actual validation defect is reported.", HTTPStatus: 422},
	"EVIDENCE_HTML_MISSING":          {Code: "EVIDENCE_HTML_MISSING", Category: ErrorRepairable, SafeMessage: "HTML 内容缺少校验或渲染证据。", ModelMessage: "If current HTML is valid and only render evidence is missing or stale, render it first without a no-op edit. Repair actual static/content defects, then render the latest version.", HTTPStatus: 422},
	"ASYNC_DECK_SLIDE":               {Code: "ASYNC_DECK_SLIDE", Category: ErrorRepairable, SafeMessage: "目录与页面规格尚未同步。", ModelMessage: "Repair outline and slide spec references before finish_task.", HTTPStatus: 422},
}

var traceSensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:provider[_ ]?body|response[_ ]?body|body)\s*[:=].*$`),
	regexp.MustCompile(`(?i)\bauthorization["']?\s*[:=]\s*["']?(?:bearer\s+)?[^"'\s,;}]+["']?`),
	regexp.MustCompile(`(?i)\b(?:api[_-]?key|key|token|secret)["']?\s*[:=]\s*["']?[^"'\s,;}]+["']?`),
	regexp.MustCompile(`(?i)\bsk-[a-z0-9._-]+\b`),
}

type AgentError struct {
	Code        string
	Category    ErrorCategory
	Operation   string
	Retryable   bool
	SafeMessage string
	Reason      string
	Measure     string
	Details     map[string]any
	Cause       error
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
		Reason: def.SafeMessage, Measure: def.ModelMessage, Details: map[string]any{}, Cause: cause,
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
	out := make(map[string]any, len(e.Details)+4)
	for key, value := range e.Details {
		out[key] = value
	}
	out["code"], out["category"] = e.Code, e.Category
	out["reason"], out["measure"] = e.Reason, e.Measure
	return out
}

func (e *AgentError) TraceProjection() map[string]any {
	if e == nil {
		return nil
	}
	out := e.ModelObservation()
	out["operation"] = e.Operation
	out["retryable"] = e.ShouldAutoRetry()
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
