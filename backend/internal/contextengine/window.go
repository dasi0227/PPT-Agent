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
	BucketReadPPT      ContextBucket = "read_ppt"
	BucketRunCommand   ContextBucket = "run_command"
	BucketSystemPrompt ContextBucket = "system_prompt"
	BucketUserPrompt   ContextBucket = "user_prompt"
	BucketChatHistory  ContextBucket = "chat_history"
	BucketUploadedFile ContextBucket = "uploaded_file"
	BucketOther        ContextBucket = "other"
)

var ContextBuckets = [...]ContextBucket{
	BucketReadPPT,
	BucketRunCommand,
	BucketSystemPrompt,
	BucketUserPrompt,
	BucketChatHistory,
	BucketUploadedFile,
	BucketOther,
}

type ContextLayer string

const (
	LayerSeed       ContextLayer = "seed"
	LayerTranscript ContextLayer = "transcript"
)

type WindowBucketDetail struct {
	Name   string       `json:"name"`
	Source string       `json:"source"`
	Layer  ContextLayer `json:"layer"`
	Tokens int          `json:"tokens"`
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
	rawBuckets := emptyBuckets()
	details := emptyDetails()
	add := func(bucket ContextBucket, name, source string, layer ContextLayer, tokens int) {
		if tokens <= 0 {
			return
		}
		rawBuckets[bucket] += tokens
		details[bucket] = append(details[bucket], WindowBucketDetail{
			Name: name, Source: source, Layer: layer, Tokens: tokens,
		})
	}

	add(BucketSystemPrompt, "system policy", "system_prompt", LayerSeed, EstimateTextTokens(input.System)+messageEnvelopeTokens)
	remainingUser := input.User
	for _, section := range []struct {
		name   string
		bucket ContextBucket
	}{
		{name: "user_instruction", bucket: BucketUserPrompt},
		{name: "run_command", bucket: BucketRunCommand},
		{name: "project_context", bucket: BucketReadPPT},
		{name: "target_context", bucket: BucketReadPPT},
		{name: "related_context", bucket: BucketReadPPT},
		{name: "design_context", bucket: BucketReadPPT},
		{name: "theme_context", bucket: BucketReadPPT},
		{name: "memory", bucket: BucketChatHistory},
		{name: "transcript", bucket: BucketChatHistory},
	} {
		content, rest := extractXMLSections(remainingUser, section.name)
		remainingUser = rest
		if content != "" {
			add(section.bucket, section.name, section.name, LayerSeed, EstimateTextTokens(content))
		}
	}
	add(BucketOther, "runtime wrapper", "runtime_input", LayerSeed, EstimateTextTokens(remainingUser)+messageEnvelopeTokens)

	if len(input.Tools) > 0 {
		add(BucketSystemPrompt, "tool schemas", "tools", LayerSeed, EstimateValueTokens(input.Tools))
	}
	toolNames := map[string]string{}
	lastUserIndex := -1
	for index := range input.Messages {
		if input.Messages[index].Role == llm.RoleUser {
			lastUserIndex = index
		}
	}
	for index, message := range input.Messages {
		for _, call := range message.ToolCalls {
			toolNames[call.ID] = call.Name
		}
		bucket := BucketChatHistory
		source := string(message.Role)
		if message.Role == llm.RoleUser {
			bucket = BucketUserPrompt
		}
		if message.Role == llm.RoleTool {
			source = toolNames[message.ToolCallID]
			switch source {
			case "read_ppt":
				bucket = BucketReadPPT
			case "run_command":
				bucket = BucketRunCommand
			}
		}
		addTranscriptMessage(add, message, index, bucket, source, index == lastUserIndex)
	}

	factor := input.Factor
	if factor <= 0 {
		factor = 1
	}
	buckets := emptyBuckets()
	scaledDetails := emptyDetails()
	total := 0
	for _, bucket := range ContextBuckets {
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

func addTranscriptMessage(
	add func(ContextBucket, string, string, ContextLayer, int),
	message llm.Message,
	index int,
	bucket ContextBucket,
	source string,
	isLatestUser bool,
) {
	baseTokens := messageEnvelopeTokens + EstimateValueTokens(message.ToolCalls) + EstimateTextTokens(message.ToolCallID)
	if baseTokens > 0 {
		add(bucket, transcriptMessageName(message, index), source, LayerTranscript, baseTokens)
	}
	for partIndex, part := range message.Content {
		partTokens := EstimateTextTokens(part.Text)
		if part.Type == "image" {
			partTokens = imageApproxTokens
		}
		partBucket, partSource := bucket, source
		if strings.HasPrefix(part.Text, "<selected_dom>") || strings.HasPrefix(part.Text, "<selected_dom_reference>") {
			partBucket, partSource = BucketChatHistory, "dom_selection"
			if isLatestUser {
				partBucket = BucketUserPrompt
			}
		}
		if part.Type == "image" && strings.HasPrefix(part.ImageRef, "project:") {
			partBucket, partSource = BucketUploadedFile, "message_attachment"
		}
		if source == "read_image" || attachmentDescriptionKind(part.Text) != "" {
			partBucket = BucketUploadedFile
			if source == "read_image" {
				partSource = "read_image"
			} else if attachmentDescriptionKind(part.Text) == "compacted_reference" {
				partSource = "compacted_reference"
			} else {
				partSource = "message_attachment"
			}
		}
		name := transcriptMessageName(message, index)
		if attachmentName := attachmentDescriptionName(part.Text); attachmentName != "" {
			name = attachmentName
		} else if partBucket == BucketUploadedFile && part.Type == "image" {
			name = "uploaded image " + itoa(partIndex+1)
		}
		add(partBucket, name, partSource, LayerTranscript, partTokens)
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

func attachmentDescriptionName(text string) string {
	if attachmentDescriptionKind(text) == "" {
		return ""
	}
	start := strings.Index(text, ">") + 1
	end := strings.LastIndex(text, "<")
	if start <= 0 || end <= start {
		return ""
	}
	var value struct {
		Name string `json:"name"`
	}
	if json.Unmarshal([]byte(text[start:end]), &value) != nil {
		return ""
	}
	return value.Name
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

func extractXMLSections(value, name string) (string, string) {
	open, close := "<"+name+">", "</"+name+">"
	sections := strings.Builder{}
	for {
		start := strings.Index(value, open)
		if start < 0 {
			break
		}
		endOffset := strings.Index(value[start+len(open):], close)
		if endOffset < 0 {
			break
		}
		end := start + len(open) + endOffset + len(close)
		sections.WriteString(value[start:end])
		value = value[:start] + value[end:]
	}
	return sections.String(), value
}

func transcriptMessageName(message llm.Message, index int) string {
	if message.Role == llm.RoleTool && message.ToolCallID != "" {
		return "tool observation " + message.ToolCallID
	}
	return string(message.Role) + " message " + itoa(index+1)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	index := len(buf)
	for value > 0 {
		index--
		buf[index] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[index:])
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
		out[bucket] = []WindowBucketDetail{}
	}
	return out
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
