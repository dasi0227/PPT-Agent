package model

import "fmt"

// SlideVersionTarget returns the project-scoped version target for one slide (keyed by stable slide_id).
func SlideVersionTarget(projectID, slideID string) string {
	return fmt.Sprintf("project/%s/slide-%s", projectID, slideID)
}

// DesignVersionTarget returns the project-scoped version target for common/tokens.css.
func DesignVersionTarget(projectID string) string {
	return fmt.Sprintf("project/%s/design", projectID)
}

// AssetVersionTarget returns the global asset version target.
func AssetVersionTarget(assetID string) string {
	return "asset-" + assetID
}
