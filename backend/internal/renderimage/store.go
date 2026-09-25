// Package renderimage owns the latest, project-scoped render reference for each slide.
// References are runtime cache entries, never user-supplied filesystem capabilities.
package renderimage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
)

const indexDir = ".runtime/render-index"
const maxBytes = 10 * 1024 * 1024

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
var ErrUnavailable = errors.New("rendered image is unavailable; render the slide again")

type Entry struct {
	ProjectID      string `json:"project_id"`
	SlideID        string `json:"slide_id"`
	RunID          string `json:"run_id"`
	ScreenshotID   string `json:"screenshot_id"`
	SourceHash     string `json:"source_hash"`
	DependencyHash string `json:"dependency_hash"`
	RenderedAt     int64  `json:"rendered_at"`
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
	// Keep an immutable authorization record for an already-read image even if
	// another render supersedes it before the next provider request.
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
	if err != nil || entry.SlideID != slideID {
		return Entry{}, ErrUnavailable
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
		return Entry{}, ErrUnavailable
	}
	file, err := os.Open(path)
	if err != nil {
		return Entry{}, ErrUnavailable
	}
	defer file.Close()
	var entry Entry
	if json.NewDecoder(io.LimitReader(file, 16*1024)).Decode(&entry) != nil || !entry.valid(projectID) {
		return Entry{}, ErrUnavailable
	}
	if _, err := imagePath(sandbox, entry); err != nil {
		return Entry{}, ErrUnavailable
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
	if err != nil || ref != entry.ImageRef() {
		return Entry{}, nil, ErrUnavailable
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
		return Entry{}, nil, ErrUnavailable
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil || len(raw) == 0 || len(raw) > maxBytes {
		return Entry{}, nil, ErrUnavailable
	}
	if _, err := png.DecodeConfig(bytes.NewReader(raw)); err != nil {
		return Entry{}, nil, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return Entry{}, nil, err
	}
	return entry, raw, nil
}

func imagePath(sandbox *artifactfs.Sandbox, entry Entry) (string, error) {
	path, err := sandbox.Resolve(entry.ImagePath())
	if err != nil {
		return "", ErrUnavailable
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxBytes {
		return "", ErrUnavailable
	}
	return path, nil
}
