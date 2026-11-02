package model

// InputDecision is private execution metadata. It is never a public event.
type InputDecision struct {
	InputID           string `json:"input_id"`
	Identity          string `json:"identity"`
	Fingerprint       string `json:"fingerprint"`
	ExecutionRevision int64  `json:"execution_revision"`
	Disposition       string `json:"disposition"`
	Failure           string `json:"failure,omitempty"`
	ReplyText         string `json:"reply_text,omitempty"`
	ReplyEventSeq     int64  `json:"reply_event_seq"`
}

const (
	InputTask            = "task"
	InputChat            = "chat"
	InputDisclosure      = "disclosure"
	InputBypass          = "bypass"
	InputDetectionFailed = "detection_failed"
)

func (d InputDecision) StopsRun() bool {
	return d.Disposition == InputDisclosure || d.Disposition == InputBypass || d.Disposition == InputDetectionFailed
}

func (d InputDecision) Reply() string {
	switch d.Disposition {
	case InputChat:
		if d.Failure != "" {
			return NewAgentError(d.Failure, "chat_reply", nil).SafeMessage
		}
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
