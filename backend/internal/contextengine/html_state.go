package contextengine

import (
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func loadHTMLState(workDir, slideID string) string {
	if _, err := os.ReadFile(filepath.Join(workDir, filepath.FromSlash(model.SlideHTMLPath(slideID)))); err != nil {
		return string(model.HTMLMissing)
	}
	return string(model.HTMLAvailable)
}
