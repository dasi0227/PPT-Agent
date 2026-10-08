package contextengine

import (
	"strings"
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

type ContextBucket string

const (
	BucketSystemPrompt ContextBucket = "system_prompt"
	BucketRuntime      ContextBucket = "runtime"
	BucketChatHistory  ContextBucket = "chat_history"
	BucketReadFile     ContextBucket = "read_file"
	BucketOther        ContextBucket = "other"
)

var ContextBuckets = [...]ContextBucket{
	BucketSystemPrompt,
	BucketRuntime,
	BucketChatHistory,
	BucketReadFile,
	BucketOther,
}

var contextWindowDetailNames = map[ContextBucket][]string{
	BucketSystemPrompt: {"system prompts", "tool definitions"},
	BucketRuntime:      {"runtime context", "runtime messages"},
	BucketChatHistory:  {"user messages", "assistant messages", "tools execution"},
	BucketReadFile:     {"read_resource", "read_image"},
	BucketOther:        {"other"},
}

type WindowBucketDetail struct {
	Name   string `json:"name"`
	Tokens int    `json:"tokens"`
}

type WindowSnapshot struct {
	Total                  int                                    `json:"total"`
	Max                    int                                    `json:"max"`
	Ratio                  float64                                `json:"ratio"`
	CompactableTokens      int                                    `json:"compactable_tokens"`
	CompactThresholdTokens int                                    `json:"compact_threshold_tokens"`
	Buckets                map[ContextBucket]int                  `json:"buckets"`
	Details                map[ContextBucket][]WindowBucketDetail `json:"details"`
}

type PromptEstimateInput struct {
	System   string
	Messages []llm.Message
	Tools    []llm.ToolSchema
	Max      int
}

type PromptEstimator struct{}

type toolWindowAttribution struct {
	bucket ContextBucket
	detail string
}

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
		details[bucket] = append(details[bucket], WindowBucketDetail{Name: name, Tokens: tokens})
	}

	if input.System != "" {
		add(BucketSystemPrompt, "system prompts", EstimateTextTokens(input.System))
		add(BucketOther, "other", messageEnvelopeTokens)
	}

	if len(input.Tools) > 0 {
		add(BucketSystemPrompt, "tool definitions", llm.EstimateToolTokens(input.Tools))
	}
	toolAttributions := map[string]toolWindowAttribution{}
	for _, message := range input.Messages {
		for _, call := range message.ToolCalls {
			bucket, detail := toolBucket(call.Name)
			toolAttributions[call.ID] = toolWindowAttribution{bucket: bucket, detail: detail}
			add(bucket, detail, EstimateValueTokens(call))
		}
		add(BucketOther, "other", messageEnvelopeTokens+EstimateTextTokens(message.ToolCallID))
		attribution := toolWindowAttribution{}
		if message.Role == llm.RoleTool {
			attribution = toolAttributions[message.ToolCallID]
		}
		for _, part := range message.Content {
			bucket, detail := messagePartBucket(message, part, attribution)
			tokens := EstimateTextTokens(part.Text)
			if part.Type == "image" {
				tokens = imageApproxTokens
			}
			add(bucket, detail, tokens)
		}
	}

	buckets := emptyBuckets()
	total := 0
	for _, bucket := range ContextBuckets {
		for _, detail := range details[bucket] {
			buckets[bucket] += detail.Tokens
		}
		total += buckets[bucket]
	}
	ratio := 0.0
	if input.Max > 0 {
		ratio = float64(total) / float64(input.Max)
	}
	return WindowSnapshot{Total: total, Max: input.Max, Ratio: ratio, Buckets: buckets, Details: details}
}

func toolBucket(name string) (ContextBucket, string) {
	switch name {
	case "read_resource":
		return BucketReadFile, "read_resource"
	case "read_image":
		return BucketReadFile, "read_image"
	default:
		return BucketChatHistory, "tools execution"
	}
}

func messagePartBucket(
	message llm.Message,
	part llm.ContentPart,
	attribution toolWindowAttribution,
) (ContextBucket, string) {
	if part.Type == "image" || attachmentDescriptionKind(part.Text) != "" {
		return BucketReadFile, "read_image"
	}
	if m := message.Metadata; m != nil && m.Origin == "runtime" {
		if m.Kind == "user_request" {
			return BucketChatHistory, "user messages"
		}
		if m.Kind == "run_image" {
			return BucketReadFile, "read_image"
		}
		if m.Kind == "summary" {
			return BucketRuntime, "runtime messages"
		}
		if m.Kind == "context" {
			return BucketRuntime, "runtime context"
		}
		if message.Role != llm.RoleTool {
			return BucketRuntime, "runtime messages"
		}
	}

	if attribution.detail != "" {
		return attribution.bucket, attribution.detail
	}
	switch message.Role {
	case llm.RoleSystem:
		return BucketSystemPrompt, "system prompts"
	case llm.RoleAssistant:
		return BucketChatHistory, "assistant messages"
	case llm.RoleUser:
		return BucketChatHistory, "user messages"
	case llm.RoleTool:
		return BucketChatHistory, "tools execution"
	default:
		return BucketOther, "other"
	}
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

func EstimateTextTokens(value string) int           { return llm.EstimateTextTokens(value) }
func EstimateValueTokens(value any) int             { return llm.EstimateValueTokens(value) }
func EstimateMessageTokens(message llm.Message) int { return llm.EstimateMessageTokens(message) }

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

type WindowStore struct {
	mu        sync.RWMutex
	snapshots map[string]WindowSnapshot
}

func NewWindowStore() *WindowStore {
	return &WindowStore{
		snapshots: map[string]WindowSnapshot{},
	}
}

func (s *WindowStore) SetSnapshot(threadID string, snapshot WindowSnapshot) {
	if s == nil || threadID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[threadID] = cloneWindowSnapshot(snapshot)
}

func (s *WindowStore) Snapshot(threadID string) (WindowSnapshot, bool) {
	if s == nil {
		return WindowSnapshot{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, ok := s.snapshots[threadID]
	return cloneWindowSnapshot(snapshot), ok
}

// SetInitialSnapshot never overwrites a newer snapshot published by a live Run.
func (s *WindowStore) SetInitialSnapshot(threadID string, snapshot WindowSnapshot) WindowSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.snapshots[threadID]; ok {
		return cloneWindowSnapshot(existing)
	}
	s.snapshots[threadID] = cloneWindowSnapshot(snapshot)
	return snapshot
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
