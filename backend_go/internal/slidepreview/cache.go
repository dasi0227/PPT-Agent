// Package slidepreview stores disposable thumbnails separately from render evidence.
package slidepreview

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/chai2010/webp"
	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"golang.org/x/image/draw"
)

var ErrUnavailable = errors.New("thumbnail is unavailable")

type entry struct {
	SourceKey      string            `json:"source_key"`
	ImageHash      string            `json:"image_hash"`
	ResourceHashes map[string]string `json:"resource_hashes"`
}

func SourceKey(htmlHash string, frame spec.RuntimeFrameContext) string {
	return spec.ContentHash([]byte("thumbnail-v1\x00" + htmlHash + "\x00" + spec.RuntimeFrameHash(frame)))
}

func InitialVersion(sourceKey string) string {
	return spec.ContentHash([]byte(sourceKey + "\x00" + renderimage.ResourceSnapshotHash(nil)))
}

// ResourceVersion is independent of the theme/frame, so applying a theme alone
// does not restart a live document. Empty resource sets need no cache buster.
func ResourceVersion(root, id string) string {
	_, cached, err := load(root, id)
	if err != nil || len(cached.ResourceHashes) == 0 {
		return ""
	}
	version, err := renderimage.ResourceHash(root, cached.ResourceHashes)
	if err != nil {
		return "unavailable"
	}
	return version
}

func Prune(root string, keep map[string]bool) {
	sandbox, err := previewSandbox(root)
	if err != nil {
		return
	}
	dir, err := sandbox.Resolve(".runtime/previews")
	if err != nil {
		return
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, file := range files {
		ext := filepath.Ext(file.Name())
		id := strings.TrimSuffix(file.Name(), ext)
		if !file.IsDir() && (ext == ".json" || ext == ".webp") && model.IsSlideID(id) && !keep[id] {
			_ = sandbox.Delete(cachePath(id, ext))
		}
	}
}

func cachePath(id, extension string) string {
	return filepath.ToSlash(filepath.Join(".runtime", "previews", id+extension))
}

func previewSandbox(root string) (*artifactfs.Sandbox, error) {
	sandbox, err := artifactfs.NewSandbox(root)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{".runtime", ".runtime/previews"} {
		path, err := sandbox.Resolve(name)
		if err != nil {
			return nil, err
		}
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, ErrUnavailable
		}
	}
	return sandbox, nil
}

func load(root, id string) (*artifactfs.Sandbox, entry, error) {
	if !model.IsSlideID(id) {
		return nil, entry{}, ErrUnavailable
	}
	sandbox, err := previewSandbox(root)
	if err != nil {
		return nil, entry{}, err
	}
	raw, err := sandbox.Read(cachePath(id, ".json"))
	var cached entry
	if err != nil || json.Unmarshal(raw, &cached) != nil || cached.SourceKey == "" || cached.ImageHash == "" || cached.ResourceHashes == nil {
		return sandbox, entry{}, ErrUnavailable
	}
	return sandbox, cached, nil
}

// Version also observes changed or newly available files from the previous render.
// A missing/corrupt cache is expendable and must never block the project snapshot.
func Version(root, id, sourceKey string) string {
	resources := renderimage.ResourceSnapshotHash(nil)
	if _, cached, err := load(root, id); err == nil && cached.SourceKey == sourceKey {
		if current, err := renderimage.ResourceHash(root, cached.ResourceHashes); err == nil {
			resources = current
		} else {
			resources = "unavailable"
		}
	}
	return spec.ContentHash([]byte(sourceKey + "\x00" + resources))
}

func Read(root, id, sourceKey string) ([]byte, error) {
	sandbox, cached, err := load(root, id)
	if err != nil || cached.SourceKey != sourceKey {
		return nil, ErrUnavailable
	}
	current, err := renderimage.ResourceHash(root, cached.ResourceHashes)
	if err != nil || current != renderimage.ResourceSnapshotHash(cached.ResourceHashes) {
		return nil, ErrUnavailable
	}
	raw, err := sandbox.Read(cachePath(id, ".webp"))
	if err != nil || len(raw) > 2<<20 || spec.ContentHash(raw) != cached.ImageHash {
		return nil, ErrUnavailable
	}
	return raw, nil
}

// Write keeps only one image and dependency index per page, using atomic file replacement.
func Write(root, id, sourceKey string, resources map[string]string, screenshot []byte) ([]byte, error) {
	if !model.IsSlideID(id) || len(screenshot) > 10<<20 {
		return nil, ErrUnavailable
	}
	config, err := png.DecodeConfig(bytes.NewReader(screenshot))
	if err != nil || config.Width != 1920 || config.Height != 1080 {
		return nil, ErrUnavailable
	}
	source, err := png.Decode(bytes.NewReader(screenshot))
	if err != nil {
		return nil, err
	}
	thumbnail := image.NewRGBA(image.Rect(0, 0, 640, 360))
	draw.CatmullRom.Scale(thumbnail, thumbnail.Bounds(), source, source.Bounds(), draw.Src, nil)
	var output bytes.Buffer
	if err := webp.Encode(&output, thumbnail, &webp.Options{Quality: 82}); err != nil {
		return nil, err
	}
	if resources == nil {
		resources = map[string]string{}
	}
	raw := output.Bytes()
	metadata, err := json.Marshal(entry{SourceKey: sourceKey, ImageHash: spec.ContentHash(raw), ResourceHashes: resources})
	if err != nil {
		return nil, err
	}
	sandbox, err := previewSandbox(root)
	if err != nil {
		return nil, err
	}
	if err := sandbox.Write(cachePath(id, ".webp"), raw); err != nil {
		return nil, err
	}
	if err := sandbox.Write(cachePath(id, ".json"), metadata); err != nil {
		return nil, err
	}
	return raw, nil
}
