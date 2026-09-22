package service

import (
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

// Refilled references must still identify the exact restored artifact. Normal
// DOM reconciliation may tolerate deleted targets; a checkpoint resend cannot
// silently retarget them or use changed bytes with an old materialization record.
func validateRestoredDOM(project model.Project, snapshot spec.ProjectContentSnapshot, selections []model.DOMSelection) error {
	for _, selection := range selections {
		raw, err := os.ReadFile(filepath.Join(project.WorkDir, model.SlideHTMLPath(selection.SlideID)))
		content, ok := snapshot.SlidesByID[selection.SlideID]
		if err != nil || spec.ContentHash(raw) != selection.HTMLHash || !ok || content.HTMLHash != selection.HTMLHash {
			return invalidRestoredReference("恢复的 DOM 引用已失效，请移除后重新选择")
		}
	}
	return nil
}
