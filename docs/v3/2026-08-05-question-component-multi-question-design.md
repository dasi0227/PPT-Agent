# 询问组件多问题交互设计

## 背景

现有 `ask_user` 只能表达一个问题，模型容易把多个可拆分选择题糅合为一个自由问答，前端也只能渲染单题卡片。本次只扩展询问协议与对应 UI，不改 Runtime、事件总线、Timeline 分组等总体架构。

## 协议

`question.asked` 只使用 `questions[]`，每个元素是一个原子问题：

- `id`: 题目 ID，组内唯一。
- `title`: 必填，问题标题。
- `description`: 可选，前端最多显示 3 行。
- `options`: 可选，最多 3 个给定选项。
- `allow_custom`: 可选；有选项时默认 `false`，无选项时强制为填空题。

顶层 `prompt/selection/options/allow_custom` 不属于当前公共事件契约，解析时直接拒绝。

`question.answered.answer` 只使用 `answers[]`：

- 单选题：提交 `question_id + selected_option_id`。
- 增强单选：若选“自定义回答”，提交 `question_id + custom_text`，与选项二选一。
- 填空题：提交 `question_id + custom_text`。

顶层 `selected_option_ids/custom_text` 不属于当前答案契约。

## 后端

- `QuestionAskedPayload` 只暴露 `Questions`，`QuestionAnswer` 只暴露 `Answers`。
- `publicQuestion` 从工具参数解析 `questions[]`，并限制选项数最多 3。
- `ValidatePublicEvent` 只校验当前 payload，阻止 HTML 文本和非法题型。
- `InputQueue` 校验所有问题必须回答；选项题不能多选；自定义回答只在 `allow_custom=true` 时接受。
- `ask_user` 工具 schema 和系统提示词明确：多问题必须拆成原子题，有选项就是单选，最多 3 个给定选项，无法枚举才用填空题。

## 前端

- 等待回答时使用特殊问题框。
- 多问题在框内水平切换，底部左侧显示 `< n / m >`，提交按钮位于底部右侧。
- 使用 `MessageCircleQuestion`；等待时绿色闪烁，提交后稳定绿色。
- 单选题最多 3 个选项；`allow_custom=true` 时追加“自定义回答”选项。
- 题目描述最多 3 行；选项描述最多 2 行，悬浮展示完整描述。
- 填空题使用固定高度文本框。
- 任一题未回答时禁用提交。
- 提交后回归普通 Timeline 行：父行“询问了 x 个问题”，子行逐题展示 `Q：/ A：`。

## 验证

- 后端：模型事件校验、输入队列校验、Runtime ask_user 测试。
- 前端：SSE 解析、Reducer、QuestionPanel、RunStore/HistoryHydrator 回放测试。
- 全量前端 lint/build 与后端相关包测试。
