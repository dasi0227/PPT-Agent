package model

import "testing"

func TestSlideVersionTargetUsesID(t *testing.T) {
	got := PresentationSlideVersionTarget("p1", "s9")
	if got != "project/p1/slide-s9" {
		t.Fatal(got)
	}
}
