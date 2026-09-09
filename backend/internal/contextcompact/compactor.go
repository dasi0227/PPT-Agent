package contextcompact

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	compactprompts "github.com/dasi0227/PPT-Agent/backend/prompts/compact"
)

const (
	compactionTimeout      = 45 * time.Second
	maxSummaryTokens       = 4000
	outputSafetyTokens     = 1024
	retainedToolRoundCount = 2
)

type Result struct {
	Messages               []llm.Message
	Summary                string
	BeforeTranscriptTokens int
	AfterTranscriptTokens  int
	DroppedInputTokens     int
}

type Compactor struct {
	provider llm.Provider
}

func New(provider llm.Provider) *Compactor {
	return &Compactor{provider: provider}
}

func (c *Compactor) Compact(ctx context.Context, messages []llm.Message) (Result, error) {
	if c == nil || c.provider == nil {
		return Result{}, errors.New("compact provider is required")
	}
	before := messageTokens(messages)
	pruned := pruneSupersededRenderImages(cloneMessages(messages))
	compressed, retained := splitTranscript(pruned)
	if len(compressed) == 0 {
		return Result{
			Messages: retained, Summary: emptySummary(),
			BeforeTranscriptTokens: before, AfterTranscriptTokens: messageTokens(retained),
		}, nil
	}

	maxInput := c.provider.Capabilities().ContextWindowTokens - maxSummaryTokens - outputSafetyTokens
	if maxInput <= 0 {
		return Result{}, errors.New("provider context window is unavailable")
	}
	dropped := 0
	for len(compressed) > 0 && compactRequestTokens(compressed) > maxInput {
		dropped += contextengine.EstimateMessageTokens(compressed[0])
		compressed = compressed[1:]
	}
	raw, err := json.Marshal(compressed)
	if err != nil {
		return Result{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, compactionTimeout)
	defer cancel()
	response, err := c.provider.Generate(requestCtx, llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: llm.TextContent(compactprompts.Load())},
			{Role: llm.RoleUser, Content: llm.TextContent("<transcript>\n" + string(raw) + "\n</transcript>")},
		},
		Reasoning: llm.ReasoningProviderDefault, MaxOutputTokens: maxSummaryTokens,
	})
	if err != nil {
		return Result{}, err
	}
	summary := strings.TrimSpace(response.Text())
	if summary == "" {
		return Result{}, errors.New("compact summary is empty")
	}
	next := append([]llm.Message{{
		Role:    llm.RoleUser,
		Content: llm.TextContent("<context_summary>\n" + summary + "\n</context_summary>"),
	}}, retained...)
	next = compactProjectAttachmentImages(next)
	return Result{
		Messages: next, Summary: summary,
		BeforeTranscriptTokens: before, AfterTranscriptTokens: messageTokens(next),
		DroppedInputTokens: dropped,
	}, nil
}

// compactProjectAttachmentImages releases visual payload tokens while retaining
// the adjacent structured attachment descriptions in the transcript.
func compactProjectAttachmentImages(messages []llm.Message) []llm.Message {
	for index := range messages {
		parts := make([]llm.ContentPart, 0, len(messages[index].Content))
		for _, part := range messages[index].Content {
			if part.Type == "image" && strings.HasPrefix(part.ImageRef, "project:") {
				continue
			}
			parts = append(parts, part)
		}
		messages[index].Content = parts
	}
	return messages
}

func splitTranscript(messages []llm.Message) (compressed, retained []llm.Message) {
	boundary := len(messages)
	rounds := 0
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role == llm.RoleAssistant && len(messages[index].ToolCalls) > 0 {
			rounds++
			boundary = index
			if rounds == retainedToolRoundCount {
				break
			}
		}
	}
	for index, message := range messages {
		if shouldRetainUser(message) || index >= boundary {
			retained = append(retained, message)
		} else {
			compressed = append(compressed, message)
		}
	}
	return compressed, retained
}

func shouldRetainUser(message llm.Message) bool {
	if message.Role != llm.RoleUser {
		return false
	}
	text := strings.TrimSpace(message.Text())
	return strings.HasPrefix(text, "<run_user_instruction ") ||
		strings.HasPrefix(text, "User steering:") ||
		strings.HasPrefix(text, "Plan revision feedback:")
}

func compactRequestTokens(messages []llm.Message) int {
	total := contextengine.EstimateTextTokens(compactprompts.Load()) + 16
	for _, message := range messages {
		total += contextengine.EstimateMessageTokens(message)
	}
	return total
}

func messageTokens(messages []llm.Message) int {
	total := 0
	for _, message := range messages {
		total += contextengine.EstimateMessageTokens(message)
	}
	return total
}

func cloneMessages(messages []llm.Message) []llm.Message {
	out := make([]llm.Message, len(messages))
	for index, message := range messages {
		out[index] = message
		out[index].Content = append([]llm.ContentPart(nil), message.Content...)
		out[index].ToolCalls = append([]llm.ToolCall(nil), message.ToolCalls...)
	}
	return out
}

func pruneSupersededRenderImages(messages []llm.Message) []llm.Message {
	seenSlides := map[string]bool{}
	for index := len(messages) - 1; index >= 0; index-- {
		message := &messages[index]
		slideID := ""
		for _, part := range message.Content {
			if part.Type != "text" {
				continue
			}
			var payload map[string]any
			if json.Unmarshal([]byte(part.Text), &payload) == nil {
				slideID, _ = payload["slide_id"].(string)
			}
		}
		if slideID == "" {
			continue
		}
		keepImage := !seenSlides[slideID]
		seenSlides[slideID] = true
		parts := make([]llm.ContentPart, 0, len(message.Content))
		for _, part := range message.Content {
			if part.Type != "image" || keepImage {
				parts = append(parts, part)
			}
		}
		message.Content = parts
	}
	return messages
}

func emptySummary() string {
	return "## 目标与意图\n\n保留现有用户指令。\n\n" +
		"## 已完成改动\n\n无可压缩的历史内容。\n\n" +
		"## 关键决策\n\n无。\n\n" +
		"## 未决问题\n\n无。\n\n" +
		"## 下一步\n\n继续当前任务。"
}
