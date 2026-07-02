package slidejson

import (
	"os"
	"path/filepath"
	"testing"
)

// 内嵌 schema MUST 与 docs 权威 schema 保持一致（DEV-RULES R1）。
func TestSchemaMatchesDocs(t *testing.T) {
	docsPath := filepath.Join("..", "..", "..", "..", "docs", "30-data-model", "slide-json.schema.json")
	docs, err := os.ReadFile(docsPath)
	if err != nil {
		t.Fatalf("read docs schema %s: %v", docsPath, err)
	}
	if string(docs) != string(schemaBytes) {
		t.Errorf("embedded slide-json.schema.json diverged from docs (source of truth); re-copy it")
	}
}

func TestValidateAcceptsValid(t *testing.T) {
	s := SlideJSON{ID: "s1", Idx: 0, Layout: "cover", Title: "封面", Bullets: []string{"a"}}
	if err := Validate(s); err != nil {
		t.Fatalf("valid slide rejected: %v", err)
	}
}

func TestValidateRejectsBadLayout(t *testing.T) {
	raw := []byte(`{"id":"s1","idx":0,"layout":"not-a-layout","title":"x"}`)
	if err := ValidateRaw(raw); err == nil {
		t.Fatal("expected schema rejection for bad layout")
	}
}

func TestValidateRejectsMissingRequired(t *testing.T) {
	raw := []byte(`{"idx":0,"layout":"cover"}`) // 缺 id、title
	if err := ValidateRaw(raw); err == nil {
		t.Fatal("expected rejection for missing required fields")
	}
}

func TestValidateRejectsAdditionalProperties(t *testing.T) {
	raw := []byte(`{"id":"s1","idx":0,"layout":"cover","title":"x","html":"<div/>"}`)
	if err := ValidateRaw(raw); err == nil {
		t.Fatal("expected rejection for additional property (e.g. html leaked into outline)")
	}
}

func TestLayoutEnumNonEmptyAndValid(t *testing.T) {
	enum := LayoutEnum()
	if len(enum) < 10 {
		t.Fatalf("layout enum too small: %d", len(enum))
	}
	if !IsValidLayout("cover") || !IsValidLayout("thanks") || !IsValidLayout("cta") {
		t.Error("expected cover/thanks/cta to be valid layouts")
	}
	if IsValidLayout("bogus") {
		t.Error("bogus layout must be invalid")
	}
}
