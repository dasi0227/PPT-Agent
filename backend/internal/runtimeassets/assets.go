// Package runtimeassets owns the versioned resources shared by every slide surface.
package runtimeassets

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
)

//go:embed base.css fonts.css font-loader.js chrome.js theme-bridge.js fonts/* examples/*.html
var files embed.FS

func mustRead(name string) []byte {
	raw, err := files.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return raw
}
func BaseCSS() []byte                     { return mustRead("base.css") }
func ChromeJS() []byte                    { return mustRead("chrome.js") }
func Read(name string) ([]byte, error)    { return files.ReadFile(name) }
func Example(name string) ([]byte, error) { return files.ReadFile("examples/" + name + ".html") }
func Hash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

var resourcesHash = sync.OnceValue(func() string {
	h := sha256.New()
	_ = fs.WalkDir(files, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.HasPrefix(path, "examples/") {
			return nil
		}
		h.Write([]byte(path))
		h.Write([]byte{0})
		h.Write(mustRead(path))
		h.Write([]byte{0})
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))
})
var themeIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
var declarations = regexp.MustCompile(`(--[a-zA-Z0-9_-]+)\s*:\s*([^;{}]+);`)
var comments = regexp.MustCompile(`(?s)/\*.*?\*/`)
var references = regexp.MustCompile(`var\((--[a-zA-Z0-9_-]+)\)`)

func Appearance(themeID string, css []byte) *designsystem.Appearance {
	tokens := map[string]string{}
	for _, match := range declarations.FindAllStringSubmatch(comments.ReplaceAllString(string(css), ""), -1) {
		if _, exists := tokens[match[1]]; !exists {
			tokens[match[1]] = strings.TrimSpace(match[2])
		}
	}
	chrome := map[string]string{}
	for _, key := range []string{"--color-caption", "--color-fg", "--font-sans", "--font-mono"} {
		value := tokens[key]
		for i := 0; i < 8 && strings.Contains(value, "var("); i++ {
			value = references.ReplaceAllStringFunc(value, func(ref string) string { return tokens[references.FindStringSubmatch(ref)[1]] })
		}
		chrome[key] = value
	}
	return &designsystem.Appearance{Hash: Hash([]byte(resourcesHash() + "\x00" + themeID + "\x00" + string(css))), ThemeCSSURL: "/api/v1/themes/" + themeID + "/css?v=" + strings.TrimPrefix(Hash(css), "sha256:"), ChromeTokens: chrome}
}

// ProjectAppearance reads the actual CSS on every dependency check, including edits under the same theme ID.
func ProjectAppearance(workDir, themeID string) (*designsystem.Appearance, error) {
	if !themeIDPattern.MatchString(themeID) {
		return nil, fmt.Errorf("invalid theme ID")
	}
	root := filepath.Clean(filepath.Join(workDir, "..", "..", "..", "assets", "themes"))
	path := filepath.Join(root, themeID, "theme.css")
	for _, candidate := range []string{root, filepath.Join(root, themeID), path} {
		info, err := os.Lstat(candidate)
		if err != nil {
			return nil, fmt.Errorf("主题 %s 不可用，请选择现有主题", themeID)
		}
		if info.Mode()&os.ModeSymlink != 0 || (candidate == path && (!info.Mode().IsRegular() || info.Size() > 64<<10)) {
			return nil, fmt.Errorf("主题 %s 的资源路径不可用，请选择现有主题", themeID)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("主题 %s 不可用，请选择现有主题: %w", themeID, err)
	}
	if missing := designsystem.LintTokens(raw); len(missing) > 0 {
		return nil, fmt.Errorf("主题 %s 缺少共享契约，请选择有效主题", themeID)
	}
	return Appearance(themeID, raw), nil
}

// Materialize creates a frozen resource copy; font binaries never cross the worker JSON protocol.
func Materialize(dir string) error {
	return fs.WalkDir(files, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.HasPrefix(path, "examples/") {
			return nil
		}
		target := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, mustRead(path), 0644)
	})
}

var cachedDir string
var cacheMu sync.Mutex

func RenderAssetDir() (string, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cachedDir != "" {
		return cachedDir, nil
	}
	dir, err := os.MkdirTemp("", "ppt-runtime-"+resourcesHash()[:12]+"-")
	if err != nil {
		return "", err
	}
	if err = Materialize(dir); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	cachedDir = dir
	return dir, nil
}
