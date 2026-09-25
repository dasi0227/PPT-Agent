package model

import "testing"

func TestAuthoringPaths(t *testing.T) {
	if SlideHTMLPath("sli_abc") != "sli_abc.html" || SpecCollectionPath != ".spec.json" {
		t.Fatal("authoring paths are not flat")
	}
}
