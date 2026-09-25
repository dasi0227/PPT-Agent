package model

import (
	"strings"
	"testing"
)

func TestPublicTextUsesPageContextAndPreservesTechnicalContent(t *testing.T) {
	c := PublicTextContext{Pages: map[string]string{"sli_market": "第 3 页"}, HiddenValues: []string{"private-project"}}
	got := PublicText("已修改 slide:sli_market:spec 和 outline.json；private-project。\n```json\n{\"project_id\":\"example\",\"key_message\":\"API\"}\n```\nHTTP_STATUS_CODE", c)
	for _, want := range []string{"第 3 页 · 页面设计稿", "目录结构", `"project_id":"example"`, "HTTP_STATUS_CODE"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	if strings.Contains(got, "private-project") || strings.Contains(got, "sli_market") {
		t.Fatal(got)
	}
	c.SourceText = "请解释 outline.json 的协议"
	if got := PublicText(".outline.json", c); got != ".outline.json" {
		t.Fatal(got)
	}
}
