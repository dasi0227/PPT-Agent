package contextengine

import (
	"math"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
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
	BucketReadFile:     {"read_resource", "read_image", "read_project"},
	BucketRunCommand:   {"run_command"},
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
	User     string
	Messages []llm.Message
	Tools    []llm.ToolSchema
	Max      int
	Factor   float64
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
	toolAttributions := map[string]toolWindowAttribution{}
	for _, message := range input.Messages {
		for _, call := range message.ToolCalls {
			bucket, detail := toolBucket(call.Name, call.Args)
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

	factor := input.Factor
	if factor <= 0 {
		factor = 1
	}
	buckets := emptyBuckets()
	scaledDetails := make(map[ContextBucket][]WindowBucketDetail, len(ContextBuckets))
	total := 0
	for _, bucket := range ContextBuckets {
		bucketDetails := make([]WindowBucketDetail, 0, len(details[bucket]))
		for _, detail := range details[bucket] {
			detail.Tokens = int(math.Ceil(float64(detail.Tokens) * factor))
			bucketDetails = append(bucketDetails, detail)
		}
		scaledDetails[bucket] = NormalizeWindowDetails(bucket, bucketDetails)
		for _, detail := range scaledDetails[bucket] {
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

func toolBucket(name string, args map[string]any) (ContextBucket, string) {
	switch name {
	case "read_resource":
		return BucketReadFile, "read_resource"
	case "read_image":
		return BucketReadFile, "read_image"
	case "run_command":
		return BucketRunCommand, runCommandDetailName(args)
	default:
		return BucketChatHistory, "other tools"
	}
}

func messagePartBucket(
	message llm.Message,
	part llm.ContentPart,
	attribution toolWindowAttribution,
) (ContextBucket, string) {
	if m := message.Metadata; m != nil && m.Origin == "runtime" {
		if m.Kind == "summary" {
			return BucketChatHistory, "context summary"
		}
		if m.Kind == "context" {
			return BucketRuntime, "runtime state"
		}
		if message.Role != llm.RoleTool {
			return BucketRuntime, "runtime messages"
		}
	}

	if attribution.detail != "" {
		return attribution.bucket, attribution.detail
	}
	if part.Type == "image" || attachmentDescriptionKind(part.Text) != "" {
		return BucketReadFile, "read_image"
	}
	switch message.Role {
	case llm.RoleSystem:
		return BucketSystemPrompt, "system prompts"
	case llm.RoleAssistant:
		return BucketChatHistory, "assistant messages"
	case llm.RoleUser:
		return BucketChatHistory, "user messages"
	case llm.RoleTool:
		return BucketChatHistory, "other tools"
	default:
		return BucketOther, "other"
	}
}

const (
	runCommandFallbackDetail = "run_command"
	otherCommandDetail       = "other command"
	maxTopCommandDetails     = 3
)

var commandDetailNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._+-]{0,63}$`)

func runCommandDetailName(args map[string]any) string {
	source, _ := args["command"].(string)
	graph, err := commandexec.Parse(source)
	if err != nil || len(graph.Groups) == 0 || len(graph.Groups[0].Commands) == 0 ||
		len(graph.Groups[0].Commands[0].Args) == 0 {
		return otherCommandDetail
	}
	executable := strings.ToLower(path.Base(strings.ReplaceAll(graph.Groups[0].Commands[0].Args[0], `\`, "/")))
	if !commandDetailNamePattern.MatchString(executable) || executable == runCommandFallbackDetail {
		return otherCommandDetail
	}
	return executable
}

// NormalizeWindowDetails preserves the fixed detail contract for five buckets
// and applies the dynamic top-three contract to run_command.
func NormalizeWindowDetails(bucket ContextBucket, details []WindowBucketDetail) []WindowBucketDetail {
	if bucket == BucketRunCommand {
		return normalizeRunCommandDetails(details)
	}
	tokensByName := make(map[string]int, len(details))
	for _, detail := range details {
		if detail.Tokens > 0 {
			tokensByName[detail.Name] += detail.Tokens
		}
	}
	normalized := make([]WindowBucketDetail, 0, len(contextWindowDetailNames[bucket]))
	for _, name := range contextWindowDetailNames[bucket] {
		normalized = append(normalized, WindowBucketDetail{Name: name, Tokens: tokensByName[name]})
	}
	return normalized
}

func normalizeRunCommandDetails(details []WindowBucketDetail) []WindowBucketDetail {
	tokensByName := make(map[string]int, len(details))
	otherTokens := 0
	for _, detail := range details {
		if detail.Tokens <= 0 || detail.Name == runCommandFallbackDetail {
			continue
		}
		if detail.Name == otherCommandDetail || !commandDetailNamePattern.MatchString(detail.Name) {
			otherTokens += detail.Tokens
			continue
		}
		tokensByName[detail.Name] += detail.Tokens
	}
	commands := make([]WindowBucketDetail, 0, len(tokensByName))
	for name, tokens := range tokensByName {
		commands = append(commands, WindowBucketDetail{Name: name, Tokens: tokens})
	}
	sort.Slice(commands, func(i, j int) bool {
		if commands[i].Tokens != commands[j].Tokens {
			return commands[i].Tokens > commands[j].Tokens
		}
		return commands[i].Name < commands[j].Name
	})
	if len(commands) > maxTopCommandDetails {
		for _, detail := range commands[maxTopCommandDetails:] {
			otherTokens += detail.Tokens
		}
		commands = commands[:maxTopCommandDetails]
	}
	if otherTokens > 0 {
		commands = append(commands, WindowBucketDetail{Name: otherCommandDetail, Tokens: otherTokens})
	}
	if len(commands) == 0 {
		return []WindowBucketDetail{{Name: runCommandFallbackDetail}}
	}
	return commands
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
