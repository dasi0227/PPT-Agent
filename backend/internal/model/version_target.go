package model

import "fmt"

// SlideHTMLVersionTarget returns the project-scoped HTML version target for one slide.
func SlideHTMLVersionTarget(projectID, slideID string) string {
	return fmt.Sprintf("project/%s/slide-html-%s", projectID, slideID)
}

func SlideSpecVersionTarget(projectID, slideID string) string {
	return fmt.Sprintf("project/%s/slide-spec-%s", projectID, slideID)
}

func OutlineVersionTarget(projectID string) string {
	return fmt.Sprintf("project/%s/outline", projectID)
}

func ManifestVersionTarget(projectID string) string {
	return fmt.Sprintf("project/%s/manifest", projectID)
}

// DesignVersionTarget returns the project-scoped design version target.
func DesignVersionTarget(projectID string) string { return projectID + ":design" }
