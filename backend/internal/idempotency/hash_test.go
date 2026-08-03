package idempotency

import "testing"

func TestCanonicalHashIgnoresMapInsertionOrderAndDetectsContentChange(t *testing.T) {
	first, err := CanonicalHash(map[string]any{
		"target":  map[string]any{"artifact": "presentation", "level": "deck"},
		"content": "dark",
	})
	if err != nil {
		t.Fatal(err)
	}
	same, err := CanonicalHash(map[string]any{
		"content": "dark",
		"target":  map[string]any{"level": "deck", "artifact": "presentation"},
	})
	if err != nil {
		t.Fatal(err)
	}
	changed, err := CanonicalHash(map[string]any{
		"content": "light",
		"target":  map[string]any{"level": "deck", "artifact": "presentation"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first != same || first == changed {
		t.Fatalf("canonical hash mismatch: first=%s same=%s changed=%s", first, same, changed)
	}
}
