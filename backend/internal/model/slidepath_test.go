package model

import "testing"

func TestSlidePaths(t *testing.T) {
	if SlideDir("abc") != "slides/abc" {
		t.Fatal(SlideDir("abc"))
	}
	if SlideJSONPath("abc") != "slides/abc/slide.json" {
		t.Fatal(SlideJSONPath("abc"))
	}
	if SlideHTMLPath("abc") != "slides/abc/index.html" {
		t.Fatal(SlideHTMLPath("abc"))
	}
	if SlideVersionSnapshot("abc", 2) != "versions/presentation-slide-abc/v2.html" {
		t.Fatal(SlideVersionSnapshot("abc", 2))
	}
}
