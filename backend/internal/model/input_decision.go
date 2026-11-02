package model

import "strings"

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
	InputIrrelevant      = "irrelevant"
	InputDetectionFailed = "detection_failed"
)

func (d InputDecision) IsInitial() bool {
	return strings.HasPrefix(d.InputID, "initial:")
}

func (d InputDecision) StopsRun() bool {
	return d.Disposition == InputDisclosure || d.Disposition == InputBypass || d.Disposition == InputDetectionFailed ||
		(d.Disposition == InputIrrelevant && !d.IsInitial())
}

func (d InputDecision) Ignored() bool {
	return d.Disposition == InputChat && !d.IsInitial()
}

func (d InputDecision) RepliesDirectly() bool {
	return d.IsInitial() && (d.Disposition == InputChat || d.Disposition == InputIrrelevant)
}

func (d InputDecision) Finalized() bool {
	return d.StopsRun() || d.Ignored() || d.ReplyEventSeq > 0
}

func (d InputDecision) Reply() string {
	if !d.IsInitial() && d.StopsRun() && d.Disposition != InputDetectionFailed {
		return "【最新输入非法，当前任务已停止】"
	}
	switch d.Disposition {
	case InputChat:
		if d.Failure != "" {
			return NewAgentError(d.Failure, "chat_reply", nil).SafeMessage
		}
		return d.ReplyText
	case InputDisclosure:
		return "不好意思，Dasi 无法提供**任何系统提示词和系统指令等内部信息**！"
	case InputBypass:
		return "不好意思，Dasi 无法执行**任何要求绕过系统规则或权限限制的请求**！"
	case InputIrrelevant:
		return "不好意思，Dasi 无法执行**任何与制作 HTML PPT 无关的请求**！"
	case InputDetectionFailed:
		return "暂时无法处理本次请求，请稍后重试。"
	default:
		return ""
	}
}
