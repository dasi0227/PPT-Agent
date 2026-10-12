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
}

var ErrCompactionUnitTooLarge = errors.New("a complete compaction unit cannot fit in the model context; original context is unchanged")

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
	units, err := compressionUnits(compressed)
	if err != nil {
		return Result{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, compactionTimeout)
	defer cancel()
	session := llm.NewSubmissionSession("compact", maxSummaryTokens)
	title, summary := "", ""
	for len(units) > 0 {
		batch := []llm.Message{}
		if summary != "" {
			batch = append(batch, llm.Message{Role: llm.RoleUser, Content: llm.TextContent("<context_summary>\n" + summary + "\n</context_summary>")})
		}
		consumed := 0
		for consumed < len(units) {
			candidate := append(append([]llm.Message{}, batch...), units[consumed]...)
			if compactRequestTokens(candidate) > maxInput {
				break
			}
			batch = candidate
			consumed++
		}
		if consumed == 0 {
			return Result{}, ErrCompactionUnitTooLarge
		}
		raw, err := json.Marshal(batch)
		if err != nil {
			return Result{}, err
		}
		_, err = session.Generate(requestCtx, provider, llm.GenerateRequest{
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: llm.TextContent(prompts.MustLoad("command.compact").Body)},
				{Role: llm.RoleUser, Content: llm.TextContent("<transcript>\n" + string(raw) + "\n</transcript>")},
			},
			Tools: []llm.ToolSchema{compactContextToolSchema()}, MaxOutputTokens: maxSummaryTokens,
		}, func(response llm.GenerateResponse) error {
			var parseErr error
			title, summary, parseErr = parseCompactContextResponse(response)
			return parseErr
		}, "This context batch has not been submitted. Call compact_context alone without ordinary text, with only title and content. Preserve the supplied transcript and previous summary and follow the five-section summary contract. Rejected calls do not count as valid submissions.")
		if err != nil {
			return Result{}, err
		}
		units = units[consumed:]
	}

	next := append([]llm.Message{{
		Role:     llm.RoleUser,
		Content:  llm.TextContent("<context_summary>\n" + summary + "\n</context_summary>"),
		Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "summary"},
	}}, retained...)
	next = compactProjectAttachmentImages(next)
	next = compactDOMSelections(next)
	return Result{
		Messages: next, Title: title, Content: summary,
		BeforeTranscriptTokens: before, AfterTranscriptTokens: messageTokens(next),
	}, nil
}

func compactContextToolSchema() llm.ToolSchema {
	return commandresult.Schema(compactContextToolName,
		"Submit the durable working context for the same continuing task.",
		"Short single-line plain-text timeline title in the conversation language identifying the task, stage or key decision, without Markdown, command prefixes or a trailing period.",
		"Complete Markdown working summary for continuing the same task, with exactly five level-2 sections: 目标与意图, 已完成改动, 关键决策, 未决问题 and 下一步. Preserve explicit constraints, decisions, relevant page and attachment IDs, actual progress, evidence limits and remaining work; distinguish completed work from attempts or plans. Historical scope decisions are not new authorization.", 0)
}

func parseCompactContextResponse(response llm.GenerateResponse) (string, string, error) {
	result, err := commandresult.Parse(response, compactContextToolName, 0)
	return result.Title, result.Content, err
}

func compactDOMSelections(messages []llm.Message) []llm.Message {
	const open, close = "<selected_dom>", "</selected_dom>"
	activeRun := llm.LatestInputRunID(messages)
	for messageIndex := range messages {
		if activeRun != "" && llm.IsRunInput(messages[messageIndex], activeRun) {
			continue
		}
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
			decorations := make([]map[string]any, 0, len(selection.DecorationTargets))
			for _, target := range selection.DecorationTargets {
				decorations = append(decorations, map[string]any{"type": target.Type, "placement": target.Placement, "text": target.Text})
			}
			raw, _ := json.Marshal(map[string]any{"selection_id": selection.SelectionID, "marker_no": selection.MarkerNo, "comment": selection.Comment, "status": selection.Status, "slide_id": selection.SlideID, "html_hash": selection.HTMLHash, "dom_targets": targets, "decoration_targets": decorations})
			part.Text = "<selected_dom_reference>" + string(raw) + "</selected_dom_reference>"
		}
	}
	return messages
}

// compactProjectAttachmentImages releases visual payload tokens while retaining
// the adjacent structured attachment descriptions in the transcript.
func compactProjectAttachmentImages(messages []llm.Message) []llm.Message {
	activeRun := llm.LatestInputRunID(messages)
	for index := range messages {
		if activeRun != "" && llm.IsRunInput(messages[index], activeRun) {
			continue
		}
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
	activeRun := llm.LatestInputRunID(messages)
	keep := make([]bool, len(messages))
	for index, message := range messages {
		keep[index] = shouldRetainUser(message, activeRun) || index >= boundary
	}
	// A retained or unresolved tool call protects its entire round. No result
	// can be summarized while its originating call remains in the live context.
	for index, message := range messages {
		if len(message.ToolCalls) == 0 {
			continue
		}
		pending := map[string]bool{}
		for _, call := range message.ToolCalls {
			pending[call.ID] = true
		}
		end := index
		protected := keep[index]
		for cursor := index + 1; cursor < len(messages) && len(pending) > 0; cursor++ {
			end = cursor
			protected = protected || keep[cursor]
			if messages[cursor].Role == llm.RoleTool {
				delete(pending, messages[cursor].ToolCallID)
			}
		}
		if len(pending) > 0 {
			protected = true
			end = len(messages) - 1
		}
		if protected {
			for cursor := index; cursor <= end; cursor++ {
				keep[cursor] = true
			}
		}
	}
	for index, message := range messages {
		if keep[index] {
			retained = append(retained, message)
		} else {
			compressed = append(compressed, message)
		}
	}

	return compressed, retained
}

// compressionUnits preserves complete assistant-call/result rounds. Invalid
// histories fail rather than handing an orphaned result to the summarizer.
func compressionUnits(messages []llm.Message) ([][]llm.Message, error) {
	units := [][]llm.Message{}
	for index := 0; index < len(messages); {
		start := index
		message := messages[index]
		if message.Role == llm.RoleTool {
			return nil, errors.New("cannot compact an orphaned tool result")
		}
		pending := map[string]bool{}
		for _, call := range message.ToolCalls {
			if call.ID == "" || pending[call.ID] {
				return nil, errors.New("invalid tool call identity in compaction")
			}
			pending[call.ID] = true
		}
		index++
		for len(pending) > 0 && index < len(messages) {
			result := messages[index]
			if result.Role != llm.RoleTool || !pending[result.ToolCallID] {
				return nil, errors.New("incomplete tool round in compaction")
			}
			delete(pending, result.ToolCallID)
			index++
		}
		if len(pending) > 0 {
			return nil, errors.New("unresolved tool round cannot be compacted")
		}
		units = append(units, messages[start:index])
	}
	return units, nil
}

func shouldRetainUser(message llm.Message, activeRun string) bool {
	return llm.IsRunInput(message, activeRun)
}

func compactRequestTokens(messages []llm.Message) int {
	total := contextengine.EstimateTextTokens(prompts.MustLoad("command.compact").Body) +
		llm.EstimateToolTokens([]llm.ToolSchema{compactContextToolSchema()}) + 16
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
	compressed, _ := splitTranscript(llm.NormalizeHistory(messages))
	return messageTokens(compressed)
}

func emptySummary() string {
	return "## 目标与意图\n\n保留现有用户指令。\n\n" +
		"## 已完成改动\n\n无可压缩的历史内容。\n\n" +
		"## 关键决策\n\n无。\n\n" +
		"## 未决问题\n\n无。\n\n" +
		"## 下一步\n\n继续当前任务。"
}
