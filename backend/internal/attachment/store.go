// Package attachment owns the project-local, verified image attachment store.
package attachment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chai2010/webp"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

const (
	MaxFileBytes = 10 * 1024 * 1024
	MaxPixels    = 40_000_000
	thumbnailMax = 512
)

var attachmentIDPattern = regexp.MustCompile(`^att_[A-Za-z0-9_-]{1,128}$`)

type ErrorCode string

const (
	TypeUnsupported    ErrorCode = "ATTACHMENT_TYPE_UNSUPPORTED"
	TooLarge           ErrorCode = "ATTACHMENT_TOO_LARGE"
	DimensionsExceeded ErrorCode = "ATTACHMENT_DIMENSIONS_EXCEEDED"
	Invalid            ErrorCode = "ATTACHMENT_INVALID"
	NotFound           ErrorCode = "ATTACHMENT_NOT_FOUND"
)

type Error struct{ Code ErrorCode }

func (e *Error) Error() string { return string(e.Code) }

// Meta is the filesystem source of truth for one image asset.
type Meta struct {
	Version       string `json:"version"`
	ID            string `json:"id"`
	ProjectID     string `json:"project_id"`
	OriginalName  string `json:"original_name"`
	MediaType     string `json:"media_type"`
	Extension     string `json:"extension"`
	SizeBytes     int64  `json:"size_bytes"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	SHA256        string `json:"sha256"`
	OriginalPath  string `json:"original_path"`
	ThumbnailPath string `json:"thumbnail_path"`
	CreatedAt     int64  `json:"created_at"`
}

func (m Meta) Reference() model.AttachmentReference {
	return model.AttachmentReference{
		ID: m.ID, OriginalName: m.OriginalName, MediaType: m.MediaType,
		Extension: m.Extension, SizeBytes: m.SizeBytes, Width: m.Width, Height: m.Height,
	}
}

func (m Meta) ImageRef(projectID, variant string) string {
	return "project:" + projectID + "/attachment:" + m.ID + "/" + variant
}

func Create(ctx context.Context, workDir, projectID, id, originalName string, source io.Reader) (Meta, error) {
	if ctx == nil {
		return Meta{}, &Error{Code: Invalid}
	}
	if ctx.Err() != nil {
		return Meta{}, context.Cause(ctx)
	}
	if !attachmentIDPattern.MatchString(id) || strings.TrimSpace(projectID) == "" {
		return Meta{}, &Error{Code: Invalid}
	}
	raw, err := readLimited(ctx, source, MaxFileBytes)
	if err != nil {
		return Meta{}, err
	}
	mediaType, extension, err := detect(raw)
	if err != nil {
		return Meta{}, err
	}
	decoded, width, height, err := decode(raw, mediaType)
	if err != nil {
		return Meta{}, &Error{Code: Invalid}
	}
	if width <= 0 || height <= 0 || int64(width)*int64(height) > MaxPixels {
		return Meta{}, &Error{Code: DimensionsExceeded}
	}

	sandbox, err := artifactfs.NewSandbox(workDir)
	if err != nil {
		return Meta{}, err
	}
	attachmentsDir, err := sandbox.Resolve("attachments")
	if err != nil {
		return Meta{}, err
	}
	if err := os.MkdirAll(attachmentsDir, 0o755); err != nil {
		return Meta{}, err
	}
	if info, err := os.Lstat(attachmentsDir); err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return Meta{}, &Error{Code: Invalid}
	}
	finalRel := filepath.ToSlash(filepath.Join("attachments", id))
	finalDir, err := sandbox.Resolve(finalRel)
	if err != nil {
		return Meta{}, err
	}
	if _, err := os.Lstat(finalDir); err == nil {
		return Meta{}, &Error{Code: Invalid}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Meta{}, err
	}
	tempDir, err := os.MkdirTemp(attachmentsDir, ".upload-")
	if err != nil {
		return Meta{}, err
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(tempDir)
		}
	}()

	thumbnail, err := makeThumbnail(decoded)
	if err != nil {
		return Meta{}, err
	}
	cleanName := displayName(originalName, extension)
	hash := sha256.Sum256(raw)
	meta := Meta{
		Version: "1.0", ID: id, ProjectID: projectID, OriginalName: cleanName,
		MediaType: mediaType, Extension: extension, SizeBytes: int64(len(raw)), Width: width, Height: height,
		SHA256: fmt.Sprintf("%x", hash), OriginalPath: filepath.ToSlash(filepath.Join(finalRel, "original."+extension)),
		ThumbnailPath: filepath.ToSlash(filepath.Join(finalRel, "thumbnail.webp")), CreatedAt: time.Now().Unix(),
	}
	metaRaw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return Meta{}, err
	}
	if err := writeAndSync(filepath.Join(tempDir, "original."+extension), raw); err != nil {
		return Meta{}, err
	}
	if err := writeAndSync(filepath.Join(tempDir, "thumbnail.webp"), thumbnail); err != nil {
		return Meta{}, err
	}
	if err := writeAndSync(filepath.Join(tempDir, "meta.json"), metaRaw); err != nil {
		return Meta{}, err
	}
	if err := syncDir(tempDir); err != nil {
		return Meta{}, err
	}
	if err := os.Rename(tempDir, finalDir); err != nil {
		return Meta{}, err
	}
	if err := syncDir(attachmentsDir); err != nil {
		return Meta{}, err
	}
	published = true
	return meta, nil
}

func Load(workDir, id string) (Meta, error) {
	if !attachmentIDPattern.MatchString(id) {
		return Meta{}, &Error{Code: NotFound}
	}
	sandbox, err := artifactfs.NewSandbox(workDir)
	if err != nil {
		return Meta{}, err
	}
	raw, err := sandbox.Read(filepath.ToSlash(filepath.Join("attachments", id, "meta.json")))
	if errors.Is(err, os.ErrNotExist) {
		return Meta{}, &Error{Code: NotFound}
	}
	if err != nil {
		return Meta{}, err
	}
	var meta Meta
	if json.Unmarshal(raw, &meta) != nil || !validMeta(meta, id) {
		return Meta{}, &Error{Code: Invalid}
	}
	return meta, nil
}

func Read(ctx context.Context, workDir, projectID, id, variant string) (Meta, []byte, error) {
	if ctx == nil {
		return Meta{}, nil, &Error{Code: Invalid}
	}
	if ctx.Err() != nil {
		return Meta{}, nil, context.Cause(ctx)
	}
	meta, err := Load(workDir, id)
	if err != nil {
		return Meta{}, nil, err
	}
	if meta.ProjectID != projectID {
		return Meta{}, nil, &Error{Code: NotFound}
	}
	var rel string
	switch variant {
	case "original":
		rel = meta.OriginalPath
	case "thumbnail":
		rel = meta.ThumbnailPath
	default:
		return Meta{}, nil, &Error{Code: Invalid}
	}
	sandbox, err := artifactfs.NewSandbox(workDir)
	if err != nil {
		return Meta{}, nil, err
	}
	raw, err := sandbox.Read(rel)
	if errors.Is(err, os.ErrNotExist) {
		return Meta{}, nil, &Error{Code: NotFound}
	}
	if err != nil {
		return Meta{}, nil, err
	}
	if len(raw) == 0 || len(raw) > MaxFileBytes {
		return Meta{}, nil, &Error{Code: Invalid}
	}
	mediaType, _, err := detect(raw)
	if err != nil || (variant == "original" && mediaType != meta.MediaType) || (variant == "thumbnail" && mediaType != "image/webp") {
		return Meta{}, nil, &Error{Code: Invalid}
	}
	if variant == "original" {
		_, width, height, decodeErr := decode(raw, mediaType)
		hash := sha256.Sum256(raw)
		if decodeErr != nil || width != meta.Width || height != meta.Height || fmt.Sprintf("%x", hash) != meta.SHA256 {
			return Meta{}, nil, &Error{Code: Invalid}
		}
	}
	return meta, raw, nil
}

func ParseImageRef(projectID, ref string) (id, variant string, ok bool) {
	prefix := "project:" + projectID + "/attachment:"
	if !strings.HasPrefix(ref, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(ref, prefix)
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || !attachmentIDPattern.MatchString(parts[0]) || (parts[1] != "original" && parts[1] != "thumbnail") {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func readLimited(ctx context.Context, source io.Reader, limit int) ([]byte, error) {
	if source == nil {
		return nil, &Error{Code: Invalid}
	}
	reader := io.LimitReader(source, int64(limit)+1)
	var out bytes.Buffer
	if _, err := io.Copy(&out, reader); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if out.Len() > limit {
		return nil, &Error{Code: TooLarge}
	}
	if out.Len() == 0 {
		return nil, &Error{Code: Invalid}
	}
	return out.Bytes(), nil
}

func detect(raw []byte) (mediaType, extension string, err error) {
	switch {
	case len(raw) >= 8 && bytes.Equal(raw[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}):
		return "image/png", "png", nil
	case len(raw) >= 3 && raw[0] == 0xff && raw[1] == 0xd8 && raw[2] == 0xff:
		return "image/jpeg", "jpg", nil
	case len(raw) >= 12 && string(raw[:4]) == "RIFF" && string(raw[8:12]) == "WEBP":
		return "image/webp", "webp", nil
	default:
		return "", "", &Error{Code: TypeUnsupported}
	}
}

func decode(raw []byte, mediaType string) (image.Image, int, int, error) {
	var (
		decoded image.Image
		err     error
	)
	switch mediaType {
	case "image/png":
		decoded, err = png.Decode(bytes.NewReader(raw))
	case "image/jpeg":
		decoded, err = jpeg.Decode(bytes.NewReader(raw))
	case "image/webp":
		decoded, err = webp.Decode(bytes.NewReader(raw))
	default:
		return nil, 0, 0, &Error{Code: TypeUnsupported}
	}
	if err != nil || decoded == nil {
		return nil, 0, 0, &Error{Code: Invalid}
	}
	bounds := decoded.Bounds()
	return decoded, bounds.Dx(), bounds.Dy(), nil
}

func makeThumbnail(source image.Image) ([]byte, error) {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, &Error{Code: Invalid}
	}
	scale := 1.0
	if width > thumbnailMax || height > thumbnailMax {
		if width >= height {
			scale = float64(thumbnailMax) / float64(width)
		} else {
			scale = float64(thumbnailMax) / float64(height)
		}
	}
	targetWidth := max(1, int(float64(width)*scale))
	targetHeight := max(1, int(float64(height)*scale))
	thumbnail := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	for y := 0; y < targetHeight; y++ {
		sourceY := bounds.Min.Y + y*height/targetHeight
		for x := 0; x < targetWidth; x++ {
			sourceX := bounds.Min.X + x*width/targetWidth
			thumbnail.Set(x, y, color.RGBAModel.Convert(source.At(sourceX, sourceY)))
		}
	}
	var out bytes.Buffer
	if err := webp.Encode(&out, thumbnail, &webp.Options{Quality: 82}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func validMeta(meta Meta, id string) bool {
	if meta.Version != "1.0" || meta.ID != id || !attachmentIDPattern.MatchString(meta.ID) || strings.TrimSpace(meta.ProjectID) == "" ||
		meta.SizeBytes <= 0 || meta.SizeBytes > MaxFileBytes || meta.Width <= 0 || meta.Height <= 0 || int64(meta.Width)*int64(meta.Height) > MaxPixels {
		return false
	}
	mediaType, extension, err := detectMetaType(meta.MediaType, meta.Extension)
	if err != nil || mediaType != meta.MediaType || extension != meta.Extension {
		return false
	}
	base := filepath.ToSlash(filepath.Join("attachments", meta.ID))
	return meta.OriginalPath == base+"/original."+meta.Extension && meta.ThumbnailPath == base+"/thumbnail.webp"
}

func detectMetaType(mediaType, extension string) (string, string, error) {
	if mediaType == "image/png" && extension == "png" || mediaType == "image/jpeg" && extension == "jpg" || mediaType == "image/webp" && extension == "webp" {
		return mediaType, extension, nil
	}
	return "", "", errors.New("invalid attachment media type")
}

func displayName(name, extension string) string {
	name = strings.TrimSpace(filepath.Base(name))
	name = strings.Map(func(value rune) rune {
		if value < 32 || value == 127 {
			return -1
		}
		return value
	}, name)
	if name == "" || name == "." {
		return "image." + extension
	}
	return name
}

func writeAndSync(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return dir.Sync()
}
