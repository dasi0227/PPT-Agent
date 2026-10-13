package model

import "testing"

func TestAuthoringPaths(t *testing.T) {
	if SlideHTMLPath("sli_abc") != "sli_abc.html" || SlideSpecPath("sli_abc") != ".sli_abc.json" {
		t.Fatal("authoring paths are not flat")
	}
	for _, path := range []string{"sli_abc.json", ".sli_.json", "slides/.sli_abc.json", "../.sli_abc.json", ".sli_abc.json.bak"} {
		if _, ok := SlideIDFromSpecPath(path); ok {
			t.Fatalf("accepted invalid Spec path %q", path)
		}
	}
	if id, ok := SlideIDFromSpecPath(".sli_abc.json"); !ok || id != "sli_abc" {
		t.Fatal("Spec path lost its stable ID")
	}
}
