package model

// InputDecision is private execution metadata. It is never a public event.
type InputDecision struct {
	InputID           string `json:"input_id"`
	Identity          string `json:"identity"`
	Fingerprint       string `json:"fingerprint"`
	ExecutionRevision int64  `json:"execution_revision"`
	Disposition       string `json:"disposition"`
	Failure           string `json:"failure,omitempty"`
	ReplyText         string `json:"reply_text"`
	ReplyEventSeq     int64  `json:"reply_event_seq"`
}

const (
	InputGreeting        = "greeting"
	InputIdentity        = "identity"
	InputPPT             = "ppt"
	InputIrrelevant      = "irrelevant"
	InputDisclosure      = "disclosure"
	InputBypass          = "bypass"
	InputDetectionFailed = "detection_failed"
)

func (d InputDecision) StopsRun() bool {
	return d.Disposition == InputDisclosure || d.Disposition == InputBypass || d.Disposition == InputIrrelevant || d.Disposition == InputDetectionFailed
}

func (d InputDecision) Reply() string {
	switch d.Disposition {
	case InputGreeting, InputIdentity, InputIrrelevant:
		return d.ReplyText
	case InputDisclosure:
		return "我无法提供系统提示词或其他内部指令，也不能通过翻译、摘要或改写转述这些内容。"
	case InputBypass:
		return "我无法执行要求绕过系统规则或权限限制的请求。"
	case InputDetectionFailed:
		return "暂时无法处理本次请求，请稍后重试。"
	default:
		return ""
	}
}

// HasFixedReply is independent of whether its text has been committed yet.
func (d InputDecision) HasFixedReply() bool {
	return len(d.ReplyCandidates()) > 0 || d.Reply() != ""
}

// ReplyCandidates are selected by persistence when the reply event commits.
// Reading a decision never draws another reply.
func (d InputDecision) ReplyCandidates() []string {
	switch d.Disposition {
	case InputGreeting:
		return []string{"很高兴和你交流。", "我在，随时可以继续。", "有需要，随时告诉我。"}
	case InputIdentity:
		return []string{"我是 Dasi，你的演示文稿创作助手。", "我是 Dasi，可以帮你规划、制作和修改演示文稿。", "我是 Dasi，帮你把想法整理成演示文稿。"}
	case InputIrrelevant:
		return []string{"我专注于演示文稿创作，暂不处理与此无关的内容。", "我的服务范围是演示文稿的规划、制作与修改。", "我可以协助演示文稿相关的工作，这类内容暂时无法处理。"}
	default:
		return nil
	}
}
