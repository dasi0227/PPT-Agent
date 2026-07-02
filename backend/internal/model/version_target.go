package model

import "fmt"

// SlideVersionTarget returns the project-scoped version target for one slide index.
func SlideVersionTarget(projectID string, idx int) string {
	return fmt.Sprintf("project/%s/slide-%03d", projectID, idx)
}

// DesignVersionTarget returns the project-scoped version target for common/tokens.css.
func DesignVersionTarget(projectID string) string {
	return fmt.Sprintf("project/%s/design", projectID)
}

// AssetVersionTarget returns the global asset version target.
func AssetVersionTarget(assetID string) string {
	return "asset-" + assetID
}
