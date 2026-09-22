package model

import "testing"

func TestSlidePaths(t *testing.T) {
	if SlideDir("abc") != "slides/abc" {
		t.Fatal(SlideDir("abc"))
	}
	if SlideSpecPath("abc") != "slides/abc/spec.json" {
		t.Fatal(SlideSpecPath("abc"))
	}
	if SlideHTMLPath("abc") != "slides/abc/index.html" {
		t.Fatal(SlideHTMLPath("abc"))
	}
}
