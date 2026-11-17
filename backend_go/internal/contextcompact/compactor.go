package contextcompact

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandresult"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	prompts "github.com/dasi0227/PPT-Agent/backend/internal/prompt"
	"github.com/dasi0227/PPT-Agent/backend/internal/tooldefinition"
)

const (
	compactionTimeout        = 45 * time.Second
	compactContextToolName   = "compact_context"
	fallbackTitle            = "整理当前任务上下文"
	MinimumCompactableTokens = 12_000
)

type Options struct {
	MainWindowTokens    int
	OutputReserveTokens int
	RunID               string
	// Measure accounts for the complete next main-Agent request, including
	// independently injected prompts/context and the tools for its phase.
	Measure func([]llm.Message) (int, error)
}

type Result struct {
	Messages []llm.Message
	Title    string
	Content  string
	Changed  bool
}

type Compactor struct{ provider llm.Provider }

func New(provider llm.Provider) *Compactor { return &Compactor{provider: provider} }

const correctionGuidance = "Submit compact_context alone with title and a complete merged content summary. Use exactly five level-two sections, translated into the conversation's primary language: Goal and Constraints / Work Progress / Key Decisions / Open Issues / Next Steps (目标与约束 / 工作进展 / 关键决策 / 未决事项 / 下一步). Give a short none statement for empty sections. Merge previous_summary with compressed_context; use retained_context only to calibrate intent and progress, without copying it. Remove obsolete conclusions, resolved blockers and completed next steps. The replacement must be smaller than the previous summary plus compressed records and fit the next main-Agent request. Rejected submissions do not modify history."

func (c *Compactor) Compact(ctx context.Context, messages []llm.Message, options Options) (Result, error) {
	partition, err := PartitionHistory(messages, options)
	if err != nil {
		return Result{}, err
	}
	if len(partition.Compressed) == 0 {
		return Result{Messages: cloneMessages(messages), Title: fallbackTitle, Content: emptySummary()}, nil
	}
	if c == nil || c.provider == nil {
		return Result{}, errors.New("compact provider is required")
	}
	provider := c.provider
	if factory, ok := provider.(interface{ Capture() (llm.Provider, error) }); ok {
		provider, err = factory.Capture()
		if err != nil {
			return Result{}, err
		}
	}
	requestCtx, cancel := context.WithTimeout(ctx, compactionTimeout)
	defer cancel()
	session := llm.NewSubmissionSession("compact")
	var result Result
	_, err = session.Generate(requestCtx, provider, llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: llm.TextContent(prompts.RenderGroup("system_prompt", prompts.MustLoad("command_compact")))},
			{Role: llm.RoleUser, Content: llm.TextContent(projectPartition(partition))},
		}, Tools: []llm.ToolSchema{tooldefinition.Get(compactContextToolName)},
	}, func(response llm.GenerateResponse) error {
		text, err := commandresult.Parse(response, compactContextToolName)
		if err != nil {
			return err
		}
		if err := validateSummary(text.Content); err != nil {
			return err
		}
		next := append([]llm.Message{summaryMessage(text.Content)}, cloneMessages(partition.Retained)...)
		next = compactProjectAttachmentImages(next)
		next = compactDOMSelections(next)
		if messageTokens(next[:1]) >= partition.ReplacedTokens {
			return llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/content", "The new summary is not smaller than the previous summary plus compressed records. Submit a shorter complete summary.")
		}
		if err := options.checkWindow(next); err != nil {
			return llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/content", err.Error()+" Submit a shorter complete summary; retained records cannot be removed by the model.")
		}
		result = Result{Messages: next, Title: text.Title, Content: text.Content, Changed: true}
		return nil
	}, correctionGuidance)
	if err != nil {
		return Result{}, err
	}
	if err := requestCtx.Err(); err != nil {
		return Result{}, err
	}
	return result, nil
}

func summaryMessage(content string) llm.Message {
	return llm.Message{Role: llm.RoleUser, Content: llm.TextContent("<compact_summary>\n" + escapeCompactText(content) + "\n</compact_summary>"), Metadata: &llm.MessageMetadata{Origin: "runtime", Kind: "summary"}}
}

func validateSummary(content string) error {
	headings, bodies := []string{}, []string{}
	fence := ""
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			if fence == "" {
				fence = marker
			} else if fence == marker {
				fence = ""
			}
		}
		if fence == "" && strings.HasPrefix(line, "## ") {
			headings = append(headings, strings.TrimSpace(strings.TrimPrefix(line, "## ")))
			bodies = append(bodies, "")
		} else if len(bodies) > 0 {
			bodies[len(bodies)-1] += line + "\n"
		} else if trimmed != "" {
			return llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/content", correctionGuidance)
		}
	}
	if len(headings) != 5 {
		return llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/content", correctionGuidance)
	}
	en := []string{"Goal and Constraints", "Work Progress", "Key Decisions", "Open Issues", "Next Steps"}
	zh := []string{"目标与约束", "工作进展", "关键决策", "未决事项", "下一步"}
	knownLanguage := headings[0] == en[0] || headings[0] == zh[0]
	for i, heading := range headings {
		if strings.TrimSpace(bodies[i]) == "" || strings.Contains(heading, "Completed Changes") || strings.Contains(heading, "已完成改动") || heading == "目标与意图" || heading == "未决问题" || knownLanguage && heading != en[i] && heading != zh[i] {
			return llm.SubmissionFailure(model.CodeToolArgumentInvalid, "/content", correctionGuidance)
		}
	}
	return nil
}

func emptySummary() string {
	return "## 目标与约束\n\n无可压缩的历史内容。\n\n## 工作进展\n\n无。\n\n## 关键决策\n\n无。\n\n## 未决事项\n\n无。\n\n## 下一步\n\n无。"
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
