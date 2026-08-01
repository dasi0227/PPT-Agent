package model

import "fmt"

// SlideVersionTarget returns the project-scoped version target for one slide (keyed by stable slide_id).
func PresentationSlideVersionTarget(projectID, slideID string) string {
	return fmt.Sprintf("project/%s/slide-%s", projectID, slideID)
}

func BlueprintSlideVersionTarget(projectID, slideID string) string {
	return fmt.Sprintf("project/%s/blueprint-slide-%s", projectID, slideID)
}

func BlueprintDeckVersionTarget(projectID string) string {
	return fmt.Sprintf("project/%s/blueprint-deck", projectID)
}

// DesignVersionTarget returns the project-scoped version target for common/tokens.css.
func DesignVersionTarget(projectID string) string {
	return fmt.Sprintf("project/%s/design", projectID)
}

// AssetVersionTarget returns the global asset version target.
func AssetVersionTarget(assetID string) string {
	return "asset-" + assetID
}
