package contextengine

import (
	"encoding/json"
	"math"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

type ContextBucket string

const (
	BucketSystemPrompt ContextBucket = "system_prompt"
	BucketRuntime      ContextBucket = "runtime"
	BucketChatHistory  ContextBucket = "chat_history"
	BucketReadFile     ContextBucket = "read_file"
	BucketRunCommand   ContextBucket = "run_command"
	BucketOther        ContextBucket = "other"
)

var ContextBuckets = [...]ContextBucket{
	BucketSystemPrompt,
	BucketRuntime,
	BucketChatHistory,
	BucketReadFile,
	BucketRunCommand,
	BucketOther,
}

var contextWindowDetailNames = map[ContextBucket][]string{
	BucketSystemPrompt: {"system prompts", "tool definitions"},
	BucketRuntime:      {"runtime state", "runtime resources", "runtime messages"},
	BucketChatHistory:  {"user messages", "assistant messages", "other tools", "context summary"},
	BucketReadFile:     {"read_ppt", "read_image", "read_project"},
	BucketRunCommand:   {"run_command"},
	BucketOther:        {"other"},
}

type WindowBucketDetail struct {
	Name   string `json:"name"`
	Tokens int    `json:"tokens"`
}

type WindowSnapshot struct {
	Total   int                                    `json:"total"`
	Max     int                                    `json:"max"`
	Ratio   float64                                `json:"ratio"`
	Buckets map[ContextBucket]int                  `json:"buckets"`
	Details map[ContextBucket][]WindowBucketDetail `json:"details"`
}

type PromptEstimateInput struct {
	System   string
	User     string
	Messages []llm.Message
	Tools    []llm.ToolSchema
	Max      int
	Factor   float64
}

type PromptEstimator struct{}

func (PromptEstimator) Estimate(input PromptEstimateInput) WindowSnapshot {
	details := emptyDetails()
	add := func(bucket ContextBucket, name string, tokens int) {
		if tokens <= 0 {
			return
		}
		for index := range details[bucket] {
			if details[bucket][index].Name == name {
				details[bucket][index].Tokens += tokens
				return
			}
		}
	}

	if input.System != "" {
		add(BucketSystemPrompt, "system prompts", EstimateTextTokens(input.System)+messageEnvelopeTokens)
	}
	remainingUser := input.User
	wrapperTokens := 0
	for _, section := range []struct {
		name, detail string
		bucket       ContextBucket
	}{
		{name: "user_instruction", bucket: BucketChatHistory, detail: "user messages"},
		{name: "run_command", bucket: BucketRuntime, detail: "runtime state"},
		{name: "runtime_state", bucket: BucketRuntime, detail: "runtime state"},
		{name: "project_context", bucket: BucketReadFile, detail: "read_project"},
		{name: "target_context", bucket: BucketReadFile, detail: "read_project"},
		{name: "related_context", bucket: BucketReadFile, detail: "read_project"},
		{name: "design_context", bucket: BucketReadFile, detail: "read_project"},
		{name: "theme_context", bucket: BucketReadFile, detail: "read_project"},
		{name: "available_resources", bucket: BucketRuntime, detail: "runtime resources"},
		{name: "available_context_refs", bucket: BucketRuntime, detail: "runtime resources"},
		{name: "active_run_skills", bucket: BucketRuntime, detail: "runtime resources"},
		{name: "referenced_components", bucket: BucketRuntime, detail: "runtime resources"},
		{name: "mentioned_pages", bucket: BucketRuntime, detail: "runtime resources"},
	} {
		content, rest, wrappers := extractXMLSectionContents(remainingUser, section.name)
		remainingUser = rest
		wrapperTokens += wrappers
		if content != "" && !(section.name == "user_instruction" && strings.TrimSpace(content) == `""`) {
			add(section.bucket, section.detail, EstimateTextTokens(content))
		}
	}
	if input.User != "" {
		add(BucketOther, "other", EstimateTextTokens(remainingUser)+wrapperTokens+messageEnvelopeTokens)
	}

	if len(input.Tools) > 0 {
		add(BucketSystemPrompt, "tool definitions", EstimateValueTokens(input.Tools))
	}
	toolNames := map[string]string{}
	for _, message := range input.Messages {
		for _, call := range message.ToolCalls {
			toolNames[call.ID] = call.Name
			bucket, detail := toolBucket(call.Name)
			add(bucket, detail, EstimateValueTokens(call))
		}
		add(BucketOther, "other", messageEnvelopeTokens+EstimateTextTokens(message.ToolCallID))
		toolName := ""
		if message.Role == llm.RoleTool {
			toolName = toolNames[message.ToolCallID]
		}
		for _, part := range message.Content {
			bucket, detail := messagePartBucket(message, part, toolName)
			tokens := EstimateTextTokens(part.Text)
			if part.Type == "image" {
				tokens = imageApproxTokens
			}
			add(bucket, detail, tokens)
		}
	}

	factor := input.Factor
	if factor <= 0 {
		factor = 1
	}
	buckets := emptyBuckets()
	scaledDetails := make(map[ContextBucket][]WindowBucketDetail, len(ContextBuckets))
	total := 0
	for _, bucket := range ContextBuckets {
		scaledDetails[bucket] = make([]WindowBucketDetail, 0, len(details[bucket]))
		for _, detail := range details[bucket] {
			detail.Tokens = int(math.Ceil(float64(detail.Tokens) * factor))
			scaledDetails[bucket] = append(scaledDetails[bucket], detail)
			buckets[bucket] += detail.Tokens
		}
		total += buckets[bucket]
	}
	ratio := 0.0
	if input.Max > 0 {
		ratio = float64(total) / float64(input.Max)
	}
	return WindowSnapshot{Total: total, Max: input.Max, Ratio: ratio, Buckets: buckets, Details: scaledDetails}
}

func toolBucket(name string) (ContextBucket, string) {
	switch name {
	case "read_ppt":
		return BucketReadFile, "read_ppt"
	case "read_image":
		return BucketReadFile, "read_image"
	case "run_command":
		return BucketRunCommand, "run_command"
	default:
		return BucketChatHistory, "other tools"
	}
}

func messagePartBucket(message llm.Message, part llm.ContentPart, toolName string) (ContextBucket, string) {
	if toolName != "" {
		return toolBucket(toolName)
	}
	if part.Type == "image" || attachmentDescriptionKind(part.Text) != "" {
		return BucketReadFile, "read_image"
	}
	text := strings.TrimSpace(part.Text)
	if strings.HasPrefix(text, "<context_summary>") {
		return BucketChatHistory, "context summary"
	}
	switch message.Role {
	case llm.RoleSystem:
		return BucketSystemPrompt, "system prompts"
	case llm.RoleAssistant:
		return BucketChatHistory, "assistant messages"
	case llm.RoleUser:
		if isRuntimeControlMessage(text) {
			return BucketRuntime, "runtime messages"
		}
		return BucketChatHistory, "user messages"
	case llm.RoleTool:
		return BucketChatHistory, "other tools"
	default:
		return BucketOther, "other"
	}
}

func isRuntimeControlMessage(text string) bool {
	return strings.HasPrefix(text, "Ordinary assistant text cannot submit a plan.") ||
		strings.HasPrefix(text, "Ordinary assistant text is not a completion signal.") ||
		strings.HasPrefix(text, "The user approved the plan. Approval is complete")
}

func attachmentDescriptionKind(text string) string {
	switch {
	case strings.HasPrefix(text, "<image_attachment>"):
		return "message_attachment"
	case strings.HasPrefix(text, "<attachment_reference>"):
		return "compacted_reference"
	default:
		return ""
	}
}

const (
	messageEnvelopeTokens = 4
	imageApproxTokens     = 1024
)

func EstimateTextTokens(value string) int {
	runes := utf8.RuneCountInString(value)
	if runes == 0 {
		return 0
	}
	return (runes+2)/3 + 1
}

func EstimateValueTokens(value any) int {
	raw, _ := json.Marshal(value)
	return EstimateTextTokens(string(raw))
}

func EstimateMessageTokens(message llm.Message) int {
	total := messageEnvelopeTokens
	for _, part := range message.Content {
		switch part.Type {
		case "image":
			total += imageApproxTokens
		default:
			total += EstimateTextTokens(part.Text)
		}
	}
	total += EstimateValueTokens(message.ToolCalls)
	total += EstimateTextTokens(message.ToolCallID)
	return total
}

func extractXMLSectionContents(value, name string) (string, string, int) {
	openPrefix, close := "<"+name, "</"+name+">"
	sections := strings.Builder{}
	wrapperTokens := 0
	searchFrom := 0
	for {
		startOffset := strings.Index(value[searchFrom:], openPrefix)
		if startOffset < 0 {
			break
		}
		start := searchFrom + startOffset
		nameEnd := start + len(openPrefix)
		if nameEnd >= len(value) || (value[nameEnd] != '>' && value[nameEnd] != ' ' && value[nameEnd] != '\t' && value[nameEnd] != '\n') {
			searchFrom = nameEnd
			continue
		}
		openEndOffset := strings.Index(value[nameEnd:], ">")
		if openEndOffset < 0 {
			break
		}
		contentStart := nameEnd + openEndOffset + 1
		endOffset := strings.Index(value[contentStart:], close)
		if endOffset < 0 {
			break
		}
		contentEnd := contentStart + endOffset
		end := contentEnd + len(close)
		content := value[contentStart:contentEnd]
		sections.WriteString(content)
		wrapperTokens += max(0, EstimateTextTokens(value[start:end])-EstimateTextTokens(content))
		value = value[:start] + value[end:]
		searchFrom = 0
	}
	return sections.String(), value, wrapperTokens
}

func emptyBuckets() map[ContextBucket]int {
	out := make(map[ContextBucket]int, len(ContextBuckets))
	for _, bucket := range ContextBuckets {
		out[bucket] = 0
	}
	return out
}

func emptyDetails() map[ContextBucket][]WindowBucketDetail {
	out := make(map[ContextBucket][]WindowBucketDetail, len(ContextBuckets))
	for _, bucket := range ContextBuckets {
		out[bucket] = make([]WindowBucketDetail, 0, len(contextWindowDetailNames[bucket]))
		for _, name := range contextWindowDetailNames[bucket] {
			out[bucket] = append(out[bucket], WindowBucketDetail{Name: name})
		}
	}
	return out
}

func ContextWindowDetailNames(bucket ContextBucket) []string {
	return append([]string(nil), contextWindowDetailNames[bucket]...)
}

type CalibrationStore struct {
	mu        sync.RWMutex
	factors   map[string]float64
	snapshots map[string]WindowSnapshot
}

func NewCalibrationStore() *CalibrationStore {
	return &CalibrationStore{
		factors:   map[string]float64{},
		snapshots: map[string]WindowSnapshot{},
	}
}

func (s *CalibrationStore) SetSnapshot(threadID string, snapshot WindowSnapshot) {
	if s == nil || threadID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[threadID] = cloneWindowSnapshot(snapshot)
}

func (s *CalibrationStore) Snapshot(threadID string) (WindowSnapshot, bool) {
	if s == nil {
		return WindowSnapshot{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, ok := s.snapshots[threadID]
	return cloneWindowSnapshot(snapshot), ok
}

func (s *CalibrationStore) Factor(threadID string) float64 {
	if s == nil {
		return 1
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if factor := s.factors[threadID]; factor > 0 {
		return factor
	}
	return 1
}

func (s *CalibrationStore) Observe(threadID string, rawEstimate, actualInput int) float64 {
	if s == nil || threadID == "" || rawEstimate <= 0 || actualInput <= 0 {
		return s.Factor(threadID)
	}
	ratio := clamp(float64(actualInput)/float64(rawEstimate), 0.5, 2)
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.factors[threadID]
	if current <= 0 {
		current = 1
	}
	const alpha = 0.2
	next := current*(1-alpha) + ratio*alpha
	s.factors[threadID] = next
	return next
}

func clamp(value, low, high float64) float64 {
	return math.Min(high, math.Max(low, value))
}

func cloneWindowSnapshot(snapshot WindowSnapshot) WindowSnapshot {
	out := snapshot
	out.Buckets = emptyBuckets()
	out.Details = emptyDetails()
	for bucket, tokens := range snapshot.Buckets {
		out.Buckets[bucket] = tokens
	}
	for bucket, details := range snapshot.Details {
		out.Details[bucket] = append([]WindowBucketDetail(nil), details...)
	}
	return out
}
