package contextcompact

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandresult"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
)

const (
	compactionTimeout        = 45 * time.Second
	maxSummaryTokens         = 4000
	outputSafetyTokens       = 1024
	retainedToolRoundCount   = 2
	compactContextToolName   = "compact_context"
	fallbackTitle            = "整理当前任务上下文"
	MinimumCompactableTokens = 12_000
)

type Result struct {
	Messages               []llm.Message
	Title                  string
	Content                string
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
	provider := c.provider
	if factory, ok := provider.(interface{ Capture() (llm.Provider, error) }); ok {
		captured, err := factory.Capture()
		if err != nil {
			return Result{}, err
		}
		provider = captured
	}
	before := messageTokens(messages)
	pruned := llm.NormalizeHistory(messages)
	compressed, retained := splitTranscript(pruned)
	if len(compressed) == 0 {
		return Result{
			Messages: retained, Title: fallbackTitle, Content: emptySummary(),
			BeforeTranscriptTokens: before, AfterTranscriptTokens: messageTokens(retained),
		}, nil
	}

	maxInput := provider.Capabilities().ContextWindowTokens - maxSummaryTokens - outputSafetyTokens
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
	response, err := provider.Generate(requestCtx, llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: llm.TextContent(prompts.MustLoad("command.compact").Body)},
			{Role: llm.RoleUser, Content: llm.TextContent("<transcript>\n" + string(raw) + "\n</transcript>")},
		},
		Tools:           []llm.ToolSchema{compactContextToolSchema()},
		MaxOutputTokens: maxSummaryTokens,
	})
	if err != nil {
		return Result{}, err
	}
	title, summary, err := parseCompactContextResponse(response)
	if err != nil {
		return Result{}, err
	}
	next := append([]llm.Message{{
		Role:    llm.RoleUser,
		Content: llm.TextContent("<context_summary>\n" + summary + "\n</context_summary>"),
	}}, retained...)
	next = compactProjectAttachmentImages(next)
	next = compactDOMSelections(next)
	return Result{
		Messages: next, Title: title, Content: summary,
		BeforeTranscriptTokens: before, AfterTranscriptTokens: messageTokens(next),
		DroppedInputTokens: dropped,
	}, nil
}

func compactContextToolSchema() llm.ToolSchema {
	return commandresult.Schema(compactContextToolName,
		"Submit the durable working context for the same continuing task.",
		"A short, task-specific plain-text timeline title in the conversation language.",
		"The complete Markdown context summary with goals, completed work, decisions, open questions and next steps.", 0)
}

func parseCompactContextResponse(response llm.GenerateResponse) (string, string, error) {
	result, err := commandresult.Parse(response, compactContextToolName, 0)
	return result.Title, result.Content, err
}

func compactDOMSelections(messages []llm.Message) []llm.Message {
	const open, close = "<selected_dom>", "</selected_dom>"
	for messageIndex := range messages {
		for partIndex := range messages[messageIndex].Content {
			part := &messages[messageIndex].Content[partIndex]
			if part.Type != "text" || !strings.HasPrefix(part.Text, open) || !strings.HasSuffix(part.Text, close) {
				continue
			}
			var selection model.DOMSelection
			if json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimPrefix(part.Text, open), close)), &selection) != nil {
				continue
			}
			targets := make([]map[string]any, 0, len(selection.DOMTargets))
			for _, target := range selection.DOMTargets {
				targets = append(targets, map[string]any{"fingerprint": target.Fingerprint, "tag": target.Tag, "text_summary": target.TextSummary, "status": target.Status})
			}
			chrome := make([]map[string]any, 0, len(selection.ChromeTargets))
			for _, target := range selection.ChromeTargets {
				chrome = append(chrome, map[string]any{"type": target.Type, "placement": target.Placement, "text": target.Text})
			}
			raw, _ := json.Marshal(map[string]any{"selection_id": selection.SelectionID, "marker_no": selection.MarkerNo, "comment": selection.Comment, "status": selection.Status, "slide_id": selection.SlideID, "html_hash": selection.HTMLHash, "dom_targets": targets, "chrome_targets": chrome})
			part.Text = "<selected_dom_reference>" + string(raw) + "</selected_dom_reference>"
		}
	}
	return messages
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
	total := contextengine.EstimateTextTokens(prompts.MustLoad("command.compact").Body) +
		contextengine.EstimateValueTokens([]llm.ToolSchema{compactContextToolSchema()}) + 16
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

// CompactableTokens measures only the older transcript that a compaction can
// replace. Retained instructions and the latest tool rounds do not make a
// manual compaction worthwhile, even though they still occupy the window.
func CompactableTokens(messages []llm.Message) int {
	pruned := llm.NormalizeHistory(messages)
	compressed, _ := splitTranscript(pruned)
	return messageTokens(compressed)
}

func emptySummary() string {
	return "## 目标与意图\n\n保留现有用户指令。\n\n" +
		"## 已完成改动\n\n无可压缩的历史内容。\n\n" +
		"## 关键决策\n\n无。\n\n" +
		"## 未决问题\n\n无。\n\n" +
		"## 下一步\n\n继续当前任务。"
}
