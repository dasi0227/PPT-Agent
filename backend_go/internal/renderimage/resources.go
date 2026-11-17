package renderimage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
)

// ResourceHash checks only files requested by the renderer, including missing
// resources so adding a previously unavailable image invalidates the screenshot.
func ResourceHash(root string, resources map[string]string) (string, error) {
	sandbox, err := artifactfs.NewSandbox(root)
	if err != nil {
		return "", err
	}
	current := make(map[string]string, len(resources))
	for name := range resources {
		path, err := sandbox.Resolve(name)
		if err != nil {
			return "", err
		}
		file, err := os.Open(path)
		if errors.Is(err, os.ErrNotExist) {
			current[name] = "missing"
			continue
		}
		if err != nil {
			return "", err
		}
		h := sha256.New()
		_, err = io.Copy(h, file)
		closeErr := file.Close()
		if err != nil {
			return "", err
		}
		if closeErr != nil {
			return "", closeErr
		}
		current[name] = "sha256:" + hex.EncodeToString(h.Sum(nil))
	}
	return ResourceSnapshotHash(current), nil
}

func ResourceSnapshotHash(resources map[string]string) string {
	if resources == nil {
		resources = map[string]string{}
	}
	raw, _ := json.Marshal(resources)
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
