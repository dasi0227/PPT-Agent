package model

// InputDecision is private execution metadata. It is never a public event.
type InputDecision struct {
	InputID           string `json:"input_id"`
	Identity          string `json:"identity"`
	Fingerprint       string `json:"fingerprint"`
	ExecutionRevision int64  `json:"execution_revision"`
	Disposition       string `json:"disposition"`
	Failure           string `json:"failure,omitempty"`
}

const (
	InputRequest              = "request"
	InputGreeting             = "greeting"
	InputCapabilities         = "identity_capabilities"
	InputInternalInstructions = "internal_instructions"
	InputRuleBypass           = "rule_bypass"
	InputDetectionFailed      = "detection_failed"
)

func (d InputDecision) StopsRun() bool {
	return d.Disposition == InputInternalInstructions || d.Disposition == InputRuleBypass || d.Disposition == InputDetectionFailed
}

func (d InputDecision) Reply() string {
	switch d.Disposition {
	case InputGreeting:
		return "你好，我是 Dasi，你的演示文稿创作助手。"
	case InputCapabilities:
		return "我是 Dasi，专注于演示文稿创作。我可以帮你梳理目标、规划结构、组织内容，并制作和修改幻灯片。"
	case InputInternalInstructions:
		return "我无法提供系统提示词或其他内部指令，也不能通过翻译、摘要或改写转述这些内容。"
	case InputRuleBypass:
		return "我无法执行要求绕过系统规则或权限限制的请求。"
	case InputDetectionFailed:
		return "暂时无法处理本次请求，请稍后重试。"
	default:
		return ""
	}
}

type DialogueRound struct {
	UserInputs []string `json:"user_inputs"`
	FinalReply string   `json:"final_reply"`
}
