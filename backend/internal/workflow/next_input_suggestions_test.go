package workflow

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeSuggestedNextInputsIsBestEffortAndBounded(t *testing.T) {
	long := strings.Repeat("页", 81)
	got := NormalizeSuggestedNextInputs([]any{
		"  优化\n第 2 页\t排版  ",
		42,
		long,
		"优化 第 2 页 排版",
		"补充演讲备注\u200b",
		"检查整套叙事",
		"不会进入第四项",
	})
	want := []string{"优化 第 2 页 排版", "补充演讲备注", "检查整套叙事"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized suggestions=%q want=%q", got, want)
	}
}

func TestNormalizeSuggestedNextInputsToleratesMalformedOptionalField(t *testing.T) {
	for _, input := range []any{nil, "not-an-array", map[string]any{"message": "still valid"}} {
		got := NormalizeSuggestedNextInputs(input)
		if got == nil || len(got) != 0 {
			t.Fatalf("malformed optional field produced %q", got)
		}
	}
}
