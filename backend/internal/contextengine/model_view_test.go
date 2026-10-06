package contextengine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestModelValueKeepsSourceTextAndStripsOnlyStructuredMetadata(t *testing.T) {
	source := `{"project_id":"author-provided text"}`
	raw, _ := json.Marshal(ModelValue(map[string]any{"project_id": "private-project", "content": source, "slide_id": "sli_a"}))
	if strings.Contains(string(raw), "private-project") || !strings.Contains(string(raw), "author-provided text") || !strings.Contains(string(raw), "sli_a") {
		t.Fatal(string(raw))
	}
}
