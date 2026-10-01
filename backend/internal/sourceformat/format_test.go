package sourceformat

import (
	"context"
	"strings"
	"testing"
)

func TestHTMLFormattingPreservesSensitiveTextAndIsStable(t *testing.T) {
	raw := []byte(`<!doctype html><html><head><style>.slide{color:red;padding:20px}</style></head><body><p>Hello <strong>world</strong>!</p><pre>  keep
    exact</pre><script>const message = "  keep  ";</script></body></html>`)
	formatted, err := HTML(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"Hello <strong>world</strong>!", "  keep\n    exact", "\"  keep  \"", "color: red;"} {
		if !strings.Contains(string(formatted), text) {
			t.Fatalf("format changed or omitted %q: %s", text, formatted)
		}
	}
	next, err := HTML(context.Background(), formatted)
	if err != nil || string(next) != string(formatted) {
		t.Fatalf("formatter is not stable: %v", err)
	}
}
