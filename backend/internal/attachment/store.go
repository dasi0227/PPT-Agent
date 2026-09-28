// Package attachment owns the project-local, verified image attachment store.
package attachment

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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

// Meta is derived from the original image; it is never persisted as a sidecar.
type Meta struct {
	ID           string `json:"id"`
	ProjectID    string `json:"project_id"`
	OriginalName string `json:"original_name"`
	MediaType    string `json:"media_type"`
	Extension    string `json:"extension"`
	SizeBytes    int64  `json:"size_bytes"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

func (m Meta) OriginalPath() string {
	return filepath.ToSlash(filepath.Join("attachments", m.ID+"."+m.Extension))
}

func (m Meta) ThumbnailPath() string {
	return filepath.ToSlash(filepath.Join("attachments", m.ID+".webp"))
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
		return Meta{}, err
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
	// A short-lived reservation prevents two uploads from publishing PNG and JPG
	// originals for the same ID. Only the original and derived thumbnail remain.
	reservation := filepath.Join(attachmentsDir, ".upload-"+id)
	if err := os.Mkdir(reservation, 0o700); err != nil {
		return Meta{}, err
	}
	defer os.RemoveAll(reservation)
	for _, ext := range []string{"png", "jpg"} {
		if _, err := os.Lstat(filepath.Join(attachmentsDir, id+"."+ext)); err == nil {
			return Meta{}, &Error{Code: Invalid}
		} else if !errors.Is(err, os.ErrNotExist) {
			return Meta{}, err
		}
	}
	meta := imageMeta(projectID, id, extension, raw, width, height)
	meta.OriginalName = displayName(originalName, extension)
	thumbnail, err := makeThumbnail(decoded)
	if err != nil {
		return Meta{}, err
	}
	if err := sandbox.Write(meta.ThumbnailPath(), thumbnail); err != nil {
		return Meta{}, err
	}
	if err := sandbox.Write(meta.OriginalPath(), raw); err != nil {
		_ = sandbox.Delete(meta.ThumbnailPath())
		return Meta{}, err
	}
	return meta, nil
}

func imageMeta(projectID, id, extension string, raw []byte, width, height int) Meta {
	mediaType := "image/png"
	if extension == "jpg" {
		mediaType = "image/jpeg"
	}
	return Meta{ID: id, ProjectID: projectID, OriginalName: id + "." + extension,
		MediaType: mediaType, Extension: extension, SizeBytes: int64(len(raw)),
		Width: width, Height: height}
}

func Load(workDir, projectID, id string) (Meta, error) {
	meta, _, _, err := loadOriginal(context.Background(), workDir, projectID, id)
	return meta, err
}

func loadOriginal(ctx context.Context, workDir, projectID, id string) (Meta, []byte, image.Image, error) {
	if !attachmentIDPattern.MatchString(id) {
		return Meta{}, nil, nil, &Error{Code: NotFound}
	}
	sandbox, err := artifactfs.NewSandbox(workDir)
	if err != nil {
		return Meta{}, nil, nil, err
	}
	var original, extension string
	for _, ext := range []string{"png", "jpg"} {
		path, err := sandbox.Resolve(filepath.Join("attachments", id+"."+ext))
		if err != nil {
			return Meta{}, nil, nil, err
		}
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return Meta{}, nil, nil, err
		}
		if !info.Mode().IsRegular() || original != "" {
			return Meta{}, nil, nil, &Error{Code: Invalid}
		}
		original, extension = path, ext
	}
	if original == "" {
		return Meta{}, nil, nil, &Error{Code: NotFound}
	}
	file, err := os.Open(original)
	if err != nil {
		return Meta{}, nil, nil, err
	}
	defer file.Close()
	raw, err := readLimited(ctx, file, MaxFileBytes)
	if err != nil {
		return Meta{}, nil, nil, err
	}
	mediaType, actualExtension, err := detect(raw)
	if err != nil || actualExtension != extension {
		return Meta{}, nil, nil, &Error{Code: Invalid}
	}
	decoded, width, height, err := decode(raw, mediaType)
	if err != nil {
		return Meta{}, nil, nil, err
	}
	return imageMeta(projectID, id, extension, raw, width, height), raw, decoded, nil
}

func Read(ctx context.Context, workDir, projectID, id, variant string) (Meta, []byte, error) {
	if ctx == nil || (variant != "original" && variant != "thumbnail") {
		return Meta{}, nil, &Error{Code: Invalid}
	}
	if ctx.Err() != nil {
		return Meta{}, nil, context.Cause(ctx)
	}
	meta, raw, decoded, err := loadOriginal(ctx, workDir, projectID, id)
	if err != nil {
		return Meta{}, nil, err
	}
	if variant == "original" {
		return meta, raw, nil
	}
	sandbox, err := artifactfs.NewSandbox(workDir)
	if err != nil {
		return Meta{}, nil, err
	}
	thumbnail, err := sandbox.Read(meta.ThumbnailPath())
	if err == nil {
		if len(thumbnail) == 0 || len(thumbnail) > MaxFileBytes {
			return Meta{}, nil, &Error{Code: Invalid}
		}
		if _, err := webp.DecodeConfig(bytes.NewReader(thumbnail)); err != nil {
			return Meta{}, nil, &Error{Code: Invalid}
		}
		return meta, thumbnail, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Meta{}, nil, err
	}
	thumbnail, err = makeThumbnail(decoded)
	if err != nil {
		return Meta{}, nil, err
	}
	if err := sandbox.Write(meta.ThumbnailPath(), thumbnail); err != nil {
		return Meta{}, nil, err
	}
	return meta, thumbnail, nil
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
	default:
		return "", "", &Error{Code: TypeUnsupported}
	}
}

func decode(raw []byte, mediaType string) (image.Image, int, int, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, 0, 0, &Error{Code: Invalid}
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > MaxPixels {
		return nil, 0, 0, &Error{Code: DimensionsExceeded}
	}
	var decoded image.Image
	switch mediaType {
	case "image/png":
		decoded, err = png.Decode(bytes.NewReader(raw))
	case "image/jpeg":
		decoded, err = jpeg.Decode(bytes.NewReader(raw))
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
