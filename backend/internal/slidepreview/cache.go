// Package slidepreview stores disposable thumbnails separately from render evidence.
package slidepreview

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"path/filepath"

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

func cachePath(id, extension string) string {
	return filepath.ToSlash(filepath.Join(".runtime", "previews", id+extension))
}

func load(root, id string) (*artifactfs.Sandbox, entry, error) {
	if !model.IsSlideID(id) {
		return nil, entry{}, ErrUnavailable
	}
	sandbox, err := artifactfs.NewSandbox(root)
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
	sandbox, err := artifactfs.NewSandbox(root)
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
