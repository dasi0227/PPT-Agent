package model

import "testing"

func TestSlideVersionTargetUsesID(t *testing.T) {
	got := SlideHTMLVersionTarget("p1", "s9")
	if got != "project/p1/slide-html-s9" {
		t.Fatal(got)
	}
}
