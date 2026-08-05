package workflow

import (
	"strings"
	"testing"
)

func TestSafeFinalMessagePreservesMarkdownBlocks(t *testing.T) {
	markdown := strings.Join([]string{
		"## 规划完成",
		"",
		"### 大纲结构",
		"",
		"| 页码 | 标题 |",
		"|---|---|",
		"| 01 | 开场 |",
		"",
		"- 下一步：进入执行模式",
	}, "\n")

	got := safeFinalMessage(markdown, StrategyPlan, 0)
	for _, want := range []string{
		"## 规划完成\n\n### 大纲结构",
		"| 页码 | 标题 |\n|---|---|\n| 01 | 开场 |",
		"\n- 下一步：进入执行模式",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("final markdown lost block structure %q in:\n%s", want, got)
		}
	}
}

func TestSafeFinalMessageKeepsLocalPathsAndHTMLAsText(t *testing.T) {
	markdown := "查看 /Users/example/project/outline.json\r\n<script>alert(1)</script>\r\n正文"
	got := safeFinalMessage(markdown, StrategyPlan, 0)
	for _, want := range []string{
		"/Users/example/project/outline.json",
		"<script>alert(1)</script>",
		"\n正文",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("final message should preserve %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\r") {
		t.Fatalf("final message should normalize CRLF to LF: %q", got)
	}
}

func TestPublicPlanDoesNotTruncateLongUIText(t *testing.T) {
	longTitle := strings.Repeat("很长的计划标题", 40)
	plan := publicPlan(Plan{
		ID: "p1", Revision: 1,
		Explanation: strings.Repeat("完整说明", 40),
		Steps:       []PlanStep{{ID: "s1", Title: longTitle, Status: PlanStepPending}},
	})

	if plan.Steps[0].Title != longTitle {
		t.Fatalf("plan step title was truncated: %q", plan.Steps[0].Title)
	}
	if strings.Contains(plan.Explanation, "…") {
		t.Fatalf("plan explanation should not be truncated: %q", plan.Explanation)
	}
}
