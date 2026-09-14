package workflow

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxSuggestedNextInputs = 3
const maxSuggestedNextInputRunes = 80

// NormalizeSuggestedNextInputs deliberately treats the finish field as
// best-effort metadata: malformed entries disappear without rejecting an
// otherwise valid final answer.
func NormalizeSuggestedNextInputs(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		if typed, typedOK := value.([]string); typedOK {
			raw = make([]any, len(typed))
			for index := range typed {
				raw[index] = typed[index]
			}
		} else {
			return []string{}
		}
	}

	out := make([]string, 0, maxSuggestedNextInputs)
	for _, item := range raw {
		candidate, ok := item.(string)
		if !ok {
			continue
		}
		candidate = normalizeSuggestedNextInput(candidate)
		if candidate == "" || utf8.RuneCountInString(candidate) > maxSuggestedNextInputRunes {
			continue
		}
		duplicate := false
		for _, existing := range out {
			if strings.EqualFold(existing, candidate) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		out = append(out, candidate)
		if len(out) == maxSuggestedNextInputs {
			break
		}
	}
	return out
}

func normalizeSuggestedNextInput(value string) string {
	var builder strings.Builder
	spacePending := false
	for _, char := range value {
		switch {
		case unicode.IsSpace(char):
			spacePending = builder.Len() > 0
		case unicode.IsControl(char) || unicode.In(char, unicode.Cf):
			continue
		default:
			if spacePending {
				builder.WriteByte(' ')
				spacePending = false
			}
			builder.WriteRune(char)
		}
	}
	return builder.String()
}
