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

	got := safeFinalMessage(markdown, "plan", 0)
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
	got := safeFinalMessage(markdown, "plan", 0)
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
		Title: strings.Repeat("完整说明", 40), Content: "完整计划正文",
		Steps: []PlanStep{{ID: "s1", Title: longTitle, Status: PlanStepPending}},
	})

	if plan.Steps[0].Title != longTitle {
		t.Fatalf("plan step title was truncated: %q", plan.Steps[0].Title)
	}
	if strings.Contains(plan.Title, "…") {
		t.Fatalf("plan title should not be truncated: %q", plan.Title)
	}
}

func TestSanitizePublicTextRedactsInternalTerms(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		absent  []string
		present []string
	}{
		{
			name:   "resource display keys",
			in:     "已写入 deck:design 与 slide:slide-01:html，并同步 deck:outline",
			absent: []string{"deck:design", "slide:slide-01:html", "deck:outline"},
		},
		{
			name:   "tool and control names",
			in:     "我调用 mutate_ppt 与 render_slide，随后 create_plan",
			absent: []string{"mutate_ppt", "render_slide", "create_plan"},
		},
		{
			name:   "runtime jargon and error codes",
			in:     "RunScope 校验触发 EVIDENCE_HTML_MISSING，未通过 completion gate",
			absent: []string{"RunScope", "EVIDENCE_HTML_MISSING", "completion gate"},
		},
		{
			name:    "leaves ordinary product prose intact",
			in:      "我已经完成了封面和第 2 页目录，并检查了排版。",
			present: []string{"封面", "第 2 页目录", "检查了排版"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizePublicText(tc.in, 0)
			for _, term := range tc.absent {
				if strings.Contains(got, term) {
					t.Fatalf("expected %q to be redacted, got: %q", term, got)
				}
			}
			for _, term := range tc.present {
				if !strings.Contains(got, term) {
					t.Fatalf("expected %q to survive redaction, got: %q", term, got)
				}
			}
		})
	}
}

func TestSensitiveCommandPreviewsAreRedacted(t *testing.T) {
	stdout, stderr := publicCommandPreviews(&CommandExecution{
		Sensitive: true,
		Stdout:    "TOKEN=secret\n",
		Stderr:    "private diagnostic",
	})
	if stdout != "[REDACTED]" || stderr != "[REDACTED]" {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
	stdout, stderr = publicCommandPreviews(&CommandExecution{
		Stdout: "\x1b[31mvisible\x1b[0m",
	})
	if stdout != "visible" || stderr != "" {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}
