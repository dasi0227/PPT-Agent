// Package renderimage owns the latest, project-scoped render reference for each slide.
// References are runtime cache entries, never user-supplied filesystem capabilities.
package renderimage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
)

const indexDir = ".runtime/render-index"
const maxBytes = 10 * 1024 * 1024
const maxPixels = 40_000_000

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
var ErrUnavailable = errors.New("rendered image is unavailable; render the slide again")
var ErrIndexInvalid = errors.New("rendered image index is invalid")
var publishMu sync.Mutex

type Entry struct {
	ProjectID            string            `json:"project_id"`
	SlideID              string            `json:"slide_id"`
	RunID                string            `json:"run_id"`
	ScreenshotID         string            `json:"screenshot_id"`
	PreviousScreenshotID string            `json:"previous_screenshot_id,omitempty"`
	SourceHash           string            `json:"source_hash"`
	DependencyHash       string            `json:"dependency_hash"`
	RenderedAt           int64             `json:"rendered_at"`
	ResourceHashes       map[string]string `json:"resource_hashes"`
	Diagnostics          json.RawMessage   `json:"diagnostics"`
}

func (e Entry) ImageRef() string {
	return "project:" + e.ProjectID + "/render:" + e.SlideID + "/" + e.ScreenshotID
}

func (e Entry) ImagePath() string {
	return filepath.ToSlash(filepath.Join(".runtime", "renders", e.RunID, e.ScreenshotID+".png"))
}

func (e Entry) valid(projectID string) bool {
	return e.ProjectID == projectID && identifier.MatchString(projectID) &&
		identifier.MatchString(e.SlideID) && identifier.MatchString(e.RunID) &&
		identifier.MatchString(e.ScreenshotID) && strings.HasPrefix(e.ScreenshotID, "shot_") && e.SourceHash != ""
}

// Publish atomically replaces one slide's index, without touching other slides.
func Publish(root string, entry Entry) error {
	publishMu.Lock()
	defer publishMu.Unlock()
	if !entry.valid(entry.ProjectID) {
		return ErrUnavailable
	}
	sandbox, err := artifactfs.NewSandbox(root)
	if err != nil {
		return err
	}
	if _, err := imagePath(sandbox, entry); err != nil {
		return err
	}
	// Re-rendering identical dependencies must not replace the real previous version.
	entry.PreviousScreenshotID = ""
	latest, err := Latest(root, entry.ProjectID, entry.SlideID)
	if err == nil {
		entry.PreviousScreenshotID = latest.PreviousScreenshotID
		if latest.DependencyHash != entry.DependencyHash {
			entry.PreviousScreenshotID = latest.ScreenshotID
		}
	} else if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, ErrUnavailable) {
		return err
	}
	// History keeps immutable screenshots; only the per-page latest index changes.
	if err := writeEntry(sandbox, filepath.Join(indexDir, "refs", entry.ScreenshotID+".json"), entry); err != nil {
		return err
	}
	return writeEntry(sandbox, filepath.Join(indexDir, entry.SlideID+".json"), entry)
}

func writeEntry(sandbox *artifactfs.Sandbox, relative string, entry Entry) error {
	path, err := sandbox.Resolve(relative)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".render-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func Latest(root, projectID, slideID string) (Entry, error) {
	if !identifier.MatchString(slideID) {
		return Entry{}, ErrUnavailable
	}
	entry, err := loadEntry(root, projectID, filepath.Join(indexDir, slideID+".json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Entry{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		return Entry{}, err
	}
	if entry.SlideID != slideID {
		return Entry{}, ErrIndexInvalid
	}
	return entry, nil
}

// Previous resolves the preceding distinct version, never an arbitrary path.
func Previous(root string, latest Entry) (Entry, error) {
	if !identifier.MatchString(latest.PreviousScreenshotID) {
		return Entry{}, ErrUnavailable
	}
	entry, err := loadEntry(root, latest.ProjectID, filepath.Join(indexDir, "refs", latest.PreviousScreenshotID+".json"))
	if err != nil {
		return Entry{}, err
	}
	if entry.SlideID != latest.SlideID || entry.ScreenshotID != latest.PreviousScreenshotID || entry.DependencyHash == latest.DependencyHash {
		return Entry{}, ErrIndexInvalid
	}
	return entry, nil
}

func loadEntry(root, projectID, relative string) (Entry, error) {
	sandbox, err := artifactfs.NewSandbox(root)
	if err != nil {
		return Entry{}, err
	}
	path, err := sandbox.Resolve(relative)
	if err != nil {
		return Entry{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return Entry{}, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, (2<<20)+1))
	if err != nil {
		return Entry{}, err
	}
	var entry Entry
	if len(raw) > 2<<20 || json.Unmarshal(raw, &entry) != nil || !entry.valid(projectID) {
		return Entry{}, ErrIndexInvalid
	}
	if _, err := imagePath(sandbox, entry); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// Read resolves a registered immutable snapshot. Tool authorization separately
// restricts new reads to current pages' latest image_path.
func Read(ctx context.Context, root, projectID, ref string) (Entry, []byte, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, nil, err
	}
	prefix := "project:" + projectID + "/render:"
	if !strings.HasPrefix(ref, prefix) {
		return Entry{}, nil, ErrUnavailable
	}
	parts := strings.Split(strings.TrimPrefix(ref, prefix), "/")
	if len(parts) != 2 || !identifier.MatchString(parts[0]) || !identifier.MatchString(parts[1]) {
		return Entry{}, nil, ErrUnavailable
	}
	entry, err := loadEntry(root, projectID, filepath.Join(indexDir, "refs", parts[1]+".json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Entry{}, nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		return Entry{}, nil, err
	}
	if ref != entry.ImageRef() {
		return Entry{}, nil, ErrIndexInvalid
	}
	sandbox, err := artifactfs.NewSandbox(root)
	if err != nil {
		return Entry{}, nil, err
	}
	path, err := imagePath(sandbox, entry)
	if err != nil {
		return Entry{}, nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Entry{}, nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		return Entry{}, nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return Entry{}, nil, err
	}
	if len(raw) == 0 {
		return Entry{}, nil, fmt.Errorf("%w: screenshot file is empty", ErrUnavailable)
	}
	if len(raw) > maxBytes {
		return Entry{}, nil, fmt.Errorf("%w: screenshot exceeds 10 MiB", ErrUnavailable)
	}
	config, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return Entry{}, nil, fmt.Errorf("%w: screenshot is not a valid PNG: %v", ErrUnavailable, err)
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > maxPixels {
		return Entry{}, nil, fmt.Errorf("%w: screenshot dimensions are invalid or exceed 40 megapixels", ErrUnavailable)
	}
	if _, err := png.Decode(bytes.NewReader(raw)); err != nil {
		return Entry{}, nil, fmt.Errorf("%w: screenshot PNG pixels are damaged: %v", ErrUnavailable, err)
	}
	if err := ctx.Err(); err != nil {
		return Entry{}, nil, err
	}
	return entry, raw, nil
}

func imagePath(sandbox *artifactfs.Sandbox, entry Entry) (string, error) {
	path, err := sandbox.Resolve(entry.ImagePath())
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: screenshot path is not a regular image file", ErrUnavailable)
	}
	if info.Size() <= 0 {
		return "", fmt.Errorf("%w: screenshot file is empty", ErrUnavailable)
	}
	if info.Size() > maxBytes {
		return "", fmt.Errorf("%w: screenshot exceeds 10 MiB", ErrUnavailable)
	}
	return path, nil
}
