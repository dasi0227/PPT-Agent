package export

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/dasi0227/PPT-Agent/backend/internal/attachment"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimehtml"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

const RuntimeVersion = "export-runtime-v1"

type SnapshotInput struct {
	ExportID, ProjectID, ProjectTitle, ProjectDir, ThemeID string
	ThemeCSS                                               []byte
}

type SnapshotError struct {
	Code    string
	Message string
	Missing []MissingSlide
}

func (e *SnapshotError) Error() string { return e.Message }

func CreateSnapshot(ctx context.Context, input SnapshotInput) (Snapshot, error) {
	if ctx.Err() != nil {
		return Snapshot{}, context.Cause(ctx)
	}
	manifestRaw, err := os.ReadFile(filepath.Join(input.ProjectDir, "manifest.json"))
	if err != nil {
		return Snapshot{}, snapshotError("EXPORT_RESOURCE_INVALID", "无法读取演示文稿信息。")
	}
	outlineRaw, err := os.ReadFile(filepath.Join(input.ProjectDir, "outline.json"))
	if err != nil {
		return Snapshot{}, snapshotError("EXPORT_RESOURCE_INVALID", "无法读取页面目录。")
	}
	designRaw, err := os.ReadFile(filepath.Join(input.ProjectDir, "design.json"))
	if err != nil {
		return Snapshot{}, snapshotError("EXPORT_RESOURCE_INVALID", "无法读取视觉设计。")
	}
	var manifest spec.Manifest
	var outline spec.Outline
	var design spec.Design
	if json.Unmarshal(manifestRaw, &manifest) != nil || spec.ValidateManifest(manifest) != nil || manifest.Canvas.AspectRatio != spec.CanvasAspectRatio {
		return Snapshot{}, snapshotError("EXPORT_RESOURCE_INVALID", "演示文稿画布无效。")
	}
	if json.Unmarshal(outlineRaw, &outline) != nil || spec.ValidateOutline(outline) != nil {
		return Snapshot{}, snapshotError("EXPORT_RESOURCE_INVALID", "页面目录无效。")
	}
	if json.Unmarshal(designRaw, &design) != nil || spec.ValidateDesign(design) != nil {
		return Snapshot{}, snapshotError("EXPORT_RESOURCE_INVALID", "视觉设计无效。")
	}
	if design.Theme != input.ThemeID {
		return Snapshot{}, snapshotError("EXPORT_THEME_UNAVAILABLE", "导出主题在快照期间发生变化。")
	}
	flat := spec.FlattenOutline(outline)
	if len(flat) == 0 {
		return Snapshot{}, snapshotError("EXPORT_SLIDES_MISSING", "演示文稿还没有页面，无法导出。")
	}
	missing := []MissingSlide{}
	for _, loc := range flat {
		if info, statErr := os.Stat(filepath.Join(input.ProjectDir, "slides", loc.Slide.SlideID, "index.html")); statErr != nil || !info.Mode().IsRegular() {
			missing = append(missing, MissingSlide{SlideID: loc.Slide.SlideID, Ordinal: loc.Ordinal, Title: loc.Slide.Title})
		}
	}
	if len(missing) > 0 {
		return Snapshot{}, &SnapshotError{Code: "EXPORT_SLIDES_MISSING", Message: fmt.Sprintf("有 %d 页尚未生成，无法导出完整演示文稿。", len(missing)), Missing: missing}
	}
	exportRoot := filepath.Join(input.ProjectDir, ".runtime", "exports", input.ExportID)
	snapshotRoot := filepath.Join(exportRoot, "snapshot")
	if err := os.MkdirAll(filepath.Join(snapshotRoot, "slides"), 0o700); err != nil {
		return Snapshot{}, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(exportRoot)
		}
	}()
	for name, raw := range map[string][]byte{"manifest.json": manifestRaw, "outline.json": outlineRaw, "design.json": designRaw} {
		if err := writeFile(filepath.Join(snapshotRoot, name), raw); err != nil {
			return Snapshot{}, err
		}
	}
	slides := make([]SlideSnapshot, 0, len(flat))
	for _, loc := range flat {
		raw, readErr := os.ReadFile(filepath.Join(input.ProjectDir, "slides", loc.Slide.SlideID, "index.html"))
		if readErr != nil {
			return Snapshot{}, readErr
		}
		normalized, normalizeErr := runtimehtml.Normalize(raw, design.Theme)
		if normalizeErr != nil {
			return Snapshot{}, snapshotError("EXPORT_RESOURCE_INVALID", "页面 HTML 无法解析。")
		}
		frame, ok := spec.BuildRuntimeFrame(manifest, outline, design, loc.Slide.SlideID)
		if !ok {
			return Snapshot{}, snapshotError("EXPORT_RESOURCE_INVALID", "页面运行框架无法构建。")
		}
		dir := filepath.Join(snapshotRoot, "slides", loc.Slide.SlideID)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return Snapshot{}, err
		}
		if err := writeFile(filepath.Join(dir, "index.html"), normalized); err != nil {
			return Snapshot{}, err
		}
		slides = append(slides, SlideSnapshot{ID: loc.Slide.SlideID, Title: loc.Slide.Title, Ordinal: loc.Ordinal, HTML: normalized, Frame: frame, FileName: fmt.Sprintf("%03d-%s.png", loc.Ordinal, SafeName(loc.Slide.Title))})
	}
	attachments, err := copyAttachments(ctx, input.ProjectDir, snapshotRoot, input.ProjectID)
	if err != nil {
		return Snapshot{}, err
	}
	baseCSS := append([]byte(nil), runtimeassets.BaseCSS()...)
	themeCSS := append([]byte(nil), input.ThemeCSS...)
	if len(baseCSS) == 0 || len(themeCSS) == 0 {
		return Snapshot{}, snapshotError("EXPORT_THEME_UNAVAILABLE", "导出主题不可用。")
	}
	digest := digestSnapshot(manifestRaw, outlineRaw, designRaw, baseCSS, themeCSS, slides, attachmentDigests(attachments, snapshotRoot))
	cleanup = false
	return Snapshot{ProjectID: input.ProjectID, ProjectTitle: input.ProjectTitle, Root: snapshotRoot, ManifestRaw: manifestRaw, OutlineRaw: outlineRaw, DesignRaw: designRaw, BaseCSS: baseCSS, ThemeCSS: themeCSS, ThemeID: design.Theme, Slides: slides, Attachments: attachments, SourceDigest: digest}, nil
}

func attachmentDigests(paths []string, root string) []string {
	values := make([]string, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			continue
		}
		sum := sha256.Sum256(raw)
		values = append(values, path+"\x00"+hex.EncodeToString(sum[:]))
	}
	return values
}

func snapshotError(code, message string) error { return &SnapshotError{Code: code, Message: message} }

func copyAttachments(ctx context.Context, projectRoot, snapshotRoot, projectID string) ([]string, error) {
	root := filepath.Join(projectRoot, "attachments")
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			ids = append(ids, entry.Name())
		}
	}
	sort.Strings(ids)
	paths := []string{}
	for _, id := range ids {
		if ctx.Err() != nil {
			return nil, context.Cause(ctx)
		}
		meta, err := attachment.Load(projectRoot, id)
		if err != nil || meta.ProjectID != projectID {
			return nil, snapshotError("EXPORT_RESOURCE_INVALID", "项目图片素材不完整。")
		}
		source := filepath.Join(projectRoot, filepath.FromSlash(meta.OriginalPath))
		info, err := os.Lstat(source)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, snapshotError("EXPORT_RESOURCE_INVALID", "项目图片素材不完整。")
		}
		rel := filepath.ToSlash(meta.OriginalPath)
		target := filepath.Join(snapshotRoot, filepath.FromSlash(rel))
		if err := copyFile(source, target); err != nil {
			return nil, err
		}
		paths = append(paths, rel)
	}
	return paths, nil
}

func digestSnapshot(parts ...any) string {
	h := sha256.New()
	for _, part := range parts {
		switch value := part.(type) {
		case []byte:
			_, _ = h.Write(value)
		case []SlideSnapshot:
			for _, slide := range value {
				io.WriteString(h, slide.ID)
				_, _ = h.Write(slide.HTML)
				raw, _ := json.Marshal(slide.Frame)
				_, _ = h.Write(raw)
			}
		case []string:
			for _, entry := range value {
				io.WriteString(h, entry)
			}
		case string:
			io.WriteString(h, value)
		}
		_, _ = h.Write([]byte{0})
	}
	io.WriteString(h, RuntimeVersion)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func SafeName(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) || strings.ContainsRune(`/\\:*?"<>|`, r) {
			b.WriteRune('-')
		} else {
			b.WriteRune(r)
		}
	}
	name := strings.Trim(strings.Join(strings.Fields(b.String()), " "), " .-")
	if name == "" {
		name = "slide"
	}
	runes := []rune(name)
	if len(runes) > 80 {
		name = string(runes[:80])
	}
	return name
}

func writeFile(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}
func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = out.Close()
		if !ok {
			_ = os.Remove(target)
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	if err = out.Sync(); err != nil {
		return err
	}
	ok = true
	return out.Close()
}
