package contextcompact

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

var (
	ErrUnclosedHistory   = errors.New("tool history is not closed; Runtime must restore its results before compaction or model replay")
	ErrProtectedTooLarge = errors.New("当前用户输入、决定或独立上下文已超过主模型可用窗口，无法通过此次历史压缩解决")
)

type Partition struct {
	PreviousSummary string
	Compressed      []llm.Message
	Retained        []llm.Message
	ReplacedTokens  int
}

type interaction struct {
	messages []llm.Message
	tokens   int
	turn     int
	mustKeep bool
	keep     bool
}

func (o Options) measure(messages []llm.Message) (int, error) {
	if o.Measure != nil {
		return o.Measure(messages)
	}
	return messageTokens(messages), nil
}

func (o Options) checkWindow(messages []llm.Message) error {
	tokens, err := o.measure(messages)
	if err != nil {
		return err
	}
	if o.MainWindowTokens <= 0 {
		return errors.New("the continuing main-Agent context window is required")
	}
	if tokens+o.OutputReserveTokens >= o.MainWindowTokens {
		return fmt.Errorf("main-Agent request needs %d tokens including output reserve; window is %d", tokens+o.OutputReserveTokens, o.MainWindowTokens)
	}
	return nil
}

func PartitionHistory(messages []llm.Message, options Options) (Partition, error) {
	var partition Partition
	if options.MainWindowTokens <= 0 {
		return partition, errors.New("the continuing main-Agent context window is required")
	}
	pruned := llm.WithoutRenderImages(llm.NormalizeHistory(cloneMessages(messages)))
	source := []llm.Message{}
	for _, message := range pruned {
		if message.Role == llm.RoleSystem {
			continue
		}
		if m := message.Metadata; m != nil && m.Origin == "runtime" && m.Kind == "summary" {
			if partition.PreviousSummary != "" {
				return partition, errors.New("multiple compact summaries in current history")
			}
			text := strings.TrimSpace(message.Text())
			if !strings.HasPrefix(text, "<compact_summary>") || !strings.HasSuffix(text, "</compact_summary>") {
				return partition, errors.New("invalid compact summary boundary")
			}
			partition.PreviousSummary = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, "<compact_summary>"), "</compact_summary>"))
			partition.ReplacedTokens += contextengine.EstimateMessageTokens(message)
			continue
		}
		source = append(source, message)
	}
	units, err := closedInteractions(source)
	if err != nil {
		return partition, err
	}
	activeRun := options.RunID
	if activeRun == "" {
		activeRun = llm.LatestInputRunID(source)
	}
	turn := 0
	protectedTokens := 0
	for i := range units {
		unit := &units[i]
		if unit.messages[0].Role == llm.RoleUser {
			turn++
		}
		unit.turn = turn
		for _, message := range unit.messages {
			unit.mustKeep = unit.mustKeep || activeRun != "" && llm.IsRunInput(message, activeRun)
		}
		unit.keep = unit.mustKeep
		if unit.mustKeep {
			protectedTokens += unit.tokens
		}
	}
	collect := func() []llm.Message {
		out := []llm.Message{summaryMessage(emptySummary())}
		for _, unit := range units {
			if unit.keep {
				out = append(out, unit.messages...)
			}
		}
		return out
	}
	if err := options.checkWindow(collect()); err != nil {
		return partition, fmt.Errorf("%w: %v", ErrProtectedTooLarge, err)
	}
	target := min(20_000, options.MainWindowTokens*16/100)
	kept := protectedTokens
	// Prefer whole user turns. If a long turn does not fit the soft target,
	// select its latest closed interactions without pinning the entire Run.
	for end := len(units) - 1; end >= 0; {
		start := end
		for start > 0 && units[start-1].turn == units[end].turn {
			start--
		}
		groupTokens := 0
		for i := start; i <= end; i++ {
			if !units[i].mustKeep {
				groupTokens += units[i].tokens
			}
		}
		if kept+groupTokens <= target {
			for i := start; i <= end; i++ {
				units[i].keep = true
			}
			kept += groupTokens
		} else {
			for i := end; i >= start; i-- {
				if units[i].mustKeep {
					continue
				}
				remaining := target - kept
				overshoot := min(1024, max(256, target/10))
				if remaining > 0 && units[i].tokens <= remaining+overshoot {
					units[i].keep = true
					kept += units[i].tokens
				}
			}
		}
		end = start - 1
	}
	// Mandatory data was checked separately. Evict optional complete units
	// when independent context/output reserve makes the full request too large.
	for {
		if err := options.checkWindow(collect()); err == nil {
			break
		}
		dropped := false
		for i := range units {
			if units[i].keep && !units[i].mustKeep {
				units[i].keep, dropped = false, true
				break
			}
		}
		if !dropped {
			return partition, ErrProtectedTooLarge
		}
	}
	for _, unit := range units {
		if unit.keep {
			partition.Retained = append(partition.Retained, unit.messages...)
		} else {
			partition.Compressed = append(partition.Compressed, unit.messages...)
			partition.ReplacedTokens += unit.tokens
		}
	}
	return partition, nil
}

// ValidateHistory never repairs protocol. Pending or ambiguous tool effects
// must be resolved by Runtime using its pending records and durable receipts.
func ValidateHistory(messages []llm.Message) error {
	_, err := closedInteractions(llm.NormalizeHistory(messages))
	return err
}

func closedInteractions(messages []llm.Message) ([]interaction, error) {
	units := []interaction{}
	seen := map[string]bool{}
	for i := 0; i < len(messages); {
		start := i
		message := messages[i]
		if message.Role == llm.RoleTool {
			return nil, fmt.Errorf("%w: orphaned result %s", ErrUnclosedHistory, message.ToolCallID)
		}
		pending := map[string]bool{}
		for _, call := range message.ToolCalls {
			if message.Role != llm.RoleAssistant || call.ID == "" || seen[call.ID] {
				return nil, fmt.Errorf("%w: invalid or repeated call ID", ErrUnclosedHistory)
			}
			pending[call.ID], seen[call.ID] = true, true
		}
		i++
		for len(pending) > 0 && i < len(messages) {
			result := messages[i]
			if result.Role != llm.RoleTool || !pending[result.ToolCallID] {
				return nil, fmt.Errorf("%w: missing result for tool call", ErrUnclosedHistory)
			}
			delete(pending, result.ToolCallID)
			i++
		}
		if len(pending) > 0 {
			return nil, ErrUnclosedHistory
		}
		unit := interaction{messages: messages[start:i]}
		unit.tokens = messageTokens(unit.messages)
		units = append(units, unit)
	}
	return units, nil
}

func CompactableTokens(messages []llm.Message, options Options) int {
	partition, err := PartitionHistory(messages, options)
	if err != nil || len(partition.Compressed) == 0 {
		return 0
	}
	return partition.ReplacedTokens
}

func messageTokens(messages []llm.Message) int {
	total := 0
	for _, message := range messages {
		total += contextengine.EstimateMessageTokens(message)
	}
	return total
}

func cloneMessages(messages []llm.Message) []llm.Message {
	out := append([]llm.Message(nil), messages...)
	for i := range out {
		out[i].Content = append([]llm.ContentPart(nil), out[i].Content...)
		out[i].ToolCalls = append([]llm.ToolCall(nil), out[i].ToolCalls...)
	}
	return out
}
