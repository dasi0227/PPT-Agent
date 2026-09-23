package llm

import "testing"

func TestTextPreservesAllTextParts(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content []ContentPart
		want    string
	}{
		{name: "empty"},
		{name: "single", content: TextContent("  keep whitespace\n"), want: "  keep whitespace\n"},
		{name: "multiple", content: []ContentPart{
			{Type: "text", Text: "回答"},
			{Type: "text", Text: "标记 1：复述选中内容"},
			{Type: "text", Text: "标记 3：保留原文"},
		}, want: "回答\n\n标记 1：复述选中内容\n\n标记 3：保留原文"},
		{name: "mixed with empty parts", content: []ContentPart{
			{Type: "text"}, {Type: "text", Text: "before"},
			{Type: "image", ImageRef: "private-image-reference"},
			{Type: "text"}, {Type: "text", Text: "after"},
		}, want: "before\n\nafter"},
		{name: "image only", content: []ContentPart{{Type: "image", ImageRef: "private-image-reference"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := (Message{Content: tc.content}).Text(); got != tc.want {
				t.Fatalf("message text = %q, want %q", got, tc.want)
			}
			if got := (GenerateResponse{Content: tc.content}).Text(); got != tc.want {
				t.Fatalf("response text = %q, want %q", got, tc.want)
			}
		})
	}
}
