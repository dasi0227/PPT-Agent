package workflow

import "github.com/dasi0227/PPT-Agent/backend/internal/llm"

const maxRepeatedReadRounds = 6
const maxReadObservations = 64

// ReadLoopState bounds repeated successful reads, including A/B/A/B loops.
// Keep fingerprints rather than another copy of tool output in checkpoints.
type ReadLoopState struct {
	ProgressHash string   `json:"progress_hash,omitempty"`
	Seen         []string `json:"seen,omitempty"`
	RepeatRounds int      `json:"repeat_rounds"`
}

func (s *ReadLoopState) syncProgress(key string) {
	if s.ProgressHash != key {
		*s = ReadLoopState{ProgressHash: key}
	}
}

func (s *ReadLoopState) observe(registry *ToolRegistry, calls []llm.ToolCall, results []ToolResult) bool {
	seen := make(map[string]bool, len(s.Seen))
	for _, key := range s.Seen {
		seen[key] = true
	}
	// Count model response rounds, not calls within one batch. A newly read
	// resource or changed result is useful information and breaks the streak.
	repeated := len(calls) > 0 && len(calls) == len(results)
	for i, call := range calls {
		desc, exists := registry.Descriptor(call.Name)
		if !exists || !desc.ReadOnly || i >= len(results) || !results[i].OK {
			repeated = false
			continue
		}
		result := results[i]
		key := hashCheckpointValue(struct {
			Name        string
			Args        map[string]any
			Result      ToolResult
			Observation string
			Parts       []llm.ContentPart
		}{call.Name, call.Args, result, result.Observation, result.ObservationParts})
		if !seen[key] {
			repeated = false
			seen[key] = true
			s.Seen = append(s.Seen, key)
		}
	}
	if len(s.Seen) > maxReadObservations {
		s.Seen = append([]string(nil), s.Seen[len(s.Seen)-maxReadObservations:]...)
	}
	if repeated {
		s.RepeatRounds++
	} else {
		s.RepeatRounds = 0
	}
	return s.RepeatRounds >= maxRepeatedReadRounds
}

func (state *RunState) readProgressHash() string {
	// Timestamps, call IDs, assistant prose, context compaction and provider
	// continuation resets do not establish progress. Plan content/status,
	// source changes, authorization and fresh render evidence do.
	var plan any
	if state.plan != nil {
		plan = struct {
			Content string
			Status  PlanStatus
			Steps   []PlanStep
		}{state.plan.ContentHash(), state.plan.Status, state.plan.Steps}
	}
	changes := map[string]string{}
	for _, change := range state.changeSet().All() {
		changes[change.Artifact.Key()] = change.AfterHash
	}
	return hashCheckpointValue([]any{state.mode, state.scope, plan, changes, state.ledger.Entries(state.changeSet()), state.renderedImages})
}
