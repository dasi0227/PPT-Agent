package model

// SpecCollectionPath is the authoring collection keyed by stable slide ID.
const SpecCollectionPath = ".spec.json"

// SlideHTMLPath is relative to the project's artifacts directory.
func SlideHTMLPath(slideID string) string { return slideID + ".html" }
