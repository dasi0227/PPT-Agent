package service

import (
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

func TestSuggestSubmissionRejectsMalformedOrRepeatedRequests(t *testing.T) {
	for _, tc := range []struct {
		name     string
		requests any
		valid    bool
	}{
		{"empty", []any{}, true},
		{"useful", []any{"优化第 2 页", "补充演讲备注"}, true},
		{"duplicate", []any{"Continue", "continue"}, false},
		{"too many", []any{"一", "二", "三", "四"}, false},
		{"too long", []any{strings.Repeat("字", 81)}, false},
		{"line break", []any{"继续\n修改"}, false},
		{"missing", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := llm.GenerateResponse{ToolCalls: []llm.ToolCall{{ID: "s", Name: "suggest_request", Args: map[string]any{"requests": tc.requests}}}}
			_, err := parseSuggestedRequests(response)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
		})
	}
}
