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
	"github.com/dasi0227/PPT-Agent/backend/internal/tooldefinition"
)

const (
	compactionTimeout        = 45 * time.Second
	retainedToolRoundCount   = 2
	compactContextToolName   = "compact_context"
	fallbackTitle            = "整理当前任务上下文"
	MinimumCompactableTokens = 12_000
)

type Result struct {
	Messages []llm.Message
	Title    string
	Content  string
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
	pruned := llm.WithoutRenderImages(llm.NormalizeHistory(messages))
	compressed, retained := splitTranscript(pruned)
	if len(compressed) == 0 {
		return Result{
			Messages: retained, Title: fallbackTitle, Content: emptySummary(),
		}, nil
	}

	if err := validateCompactTranscript(compressed); err != nil {
		return Result{}, err
	}
	raw, err := json.Marshal(compressed)
	if err != nil {
		return Result{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, compactionTimeout)
	defer cancel()
	session := llm.NewSubmissionSession("compact")
	title, summary := "", ""
	_, err = session.Generate(requestCtx, provider, llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: llm.TextContent(prompts.MustLoad("command.compact").Body)},
			{Role: llm.RoleUser, Content: llm.TextContent("<transcript>\n" + string(raw) + "\n</transcript>")},
		},
		Tools: []llm.ToolSchema{compactContextToolSchema()},
	}, func(response llm.GenerateResponse) error {
		var parseErr error
		title, summary, parseErr = parseCompactContextResponse(response)
		return parseErr
	}, "This context summary has not been submitted. Call compact_context alone without ordinary text, with only title and content. Preserve the supplied transcript and previous summary and follow the five-section summary contract. Rejected calls do not count as valid submissions.")
	if err != nil {
		return Result{}, err
	}

	next := append([]llm.Message{{
		Role:     llm.RoleUser,
		Content:  llm.TextContent("<compact_summary>\n" + summary + "\n</compact_summary>"),
		Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "summary"},
	}}, retained...)
	next = compactProjectAttachmentImages(next)
	next = compactDOMSelections(next)
	return Result{
		Messages: next, Title: title, Content: summary,
	}, nil
}

func compactContextToolSchema() llm.ToolSchema {
	return tooldefinition.Get(compactContextToolName)
}

func parseCompactContextResponse(response llm.GenerateResponse) (string, string, error) {
	result, err := commandresult.Parse(response, compactContextToolName)
	return result.Title, result.Content, err
}

func compactDOMSelections(messages []llm.Message) []llm.Message {
	const open, close = "<user_annotation>", "</user_annotation>"
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
			part.Text = "<user_annotation_reference>" + string(raw) + "</user_annotation_reference>"
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

// Validate complete assistant-call/result rounds before handing the history to
// the summarizer; an orphaned or unresolved result cannot be summarized safely.
func validateCompactTranscript(messages []llm.Message) error {
	for index := 0; index < len(messages); {
		message := messages[index]
		if message.Role == llm.RoleTool {
			return errors.New("cannot compact an orphaned tool result")
		}
		pending := map[string]bool{}
		for _, call := range message.ToolCalls {
			if call.ID == "" || pending[call.ID] {
				return errors.New("invalid tool call identity in compaction")
			}
			pending[call.ID] = true
		}
		index++
		for len(pending) > 0 && index < len(messages) {
			result := messages[index]
			if result.Role != llm.RoleTool || !pending[result.ToolCallID] {
				return errors.New("incomplete tool round in compaction")
			}
			delete(pending, result.ToolCallID)
			index++
		}
		if len(pending) > 0 {
			return errors.New("unresolved tool round cannot be compacted")
		}
	}
	return nil
}

func shouldRetainUser(message llm.Message, activeRun string) bool {
	return llm.IsRunInput(message, activeRun)
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
	compressed, _ := splitTranscript(llm.WithoutRenderImages(llm.NormalizeHistory(messages)))
	return messageTokens(compressed)
}

func emptySummary() string {
	return "## 目标与意图\n\n保留现有用户指令。\n\n" +
		"## 已完成改动\n\n无可压缩的历史内容。\n\n" +
		"## 关键决策\n\n无。\n\n" +
		"## 未决问题\n\n无。\n\n" +
		"## 下一步\n\n继续当前任务。"
}
