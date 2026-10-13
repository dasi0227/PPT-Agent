package model

import "strings"

func IsSlideID(id string) bool { return slideIDPattern.MatchString(id) }

// SlideSpecPath and SlideHTMLPath are relative to the project's artifacts directory.
// Callers obtain stable IDs from the current outline before accessing page sources.
func SlideSpecPath(slideID string) string { return "." + slideID + ".json" }
func SlideHTMLPath(slideID string) string { return slideID + ".html" }

// SlideIDFromSpecPath recognizes a page source path; it does not establish
// membership in the outline or authorize access to that page.
func SlideIDFromSpecPath(path string) (string, bool) {
	id := strings.TrimSuffix(strings.TrimPrefix(path, "."), ".json")
	return id, IsSlideID(id) && path == SlideSpecPath(id)
}
func SlideIDFromHTMLPath(path string) (string, bool) {
	id := strings.TrimSuffix(path, ".html")
	return id, IsSlideID(id) && path == SlideHTMLPath(id)
}
