// Package fileopen owns the local machine's default file-opening preference.
package fileopen

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	ErrConflict    = errors.New("文件设置已在其他窗口更新，请刷新后重试")
	ErrInvalid     = errors.New("文件设置无效")
	ErrUnsupported = errors.New("当前仅支持在 macOS 本机打开文件")
	ErrNotFound    = errors.New("文件不存在或已被移动")
	ErrPath        = errors.New("只能打开当前工作目录内的普通文件")
	ErrApplication = errors.New("所选应用不存在或不是有效的 macOS 应用，请重新选择")
	ErrPickerBusy  = errors.New("应用选择窗口已打开，请先完成选择")
)

type Method struct {
	OpenWith      string `json:"open_with"`
	CustomAppPath string `json:"custom_app_path"`
}
type Settings struct {
	Default    Method        `json:"default"`
	JSON       Method        `json:"json"`
	HTML       Method        `json:"html"`
	Revision   int64         `json:"revision"`
	CustomApps []Application `json:"custom_apps"`
}
type Application struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Builtin string `json:"builtin,omitempty"`
}
type View struct {
	Settings
	Supported bool `json:"supported"`
}

func DefaultSettings() Settings {
	return Settings{Default: Method{OpenWith: "system"}, JSON: Method{OpenWith: "inherit"}, HTML: Method{OpenWith: "inherit"}, CustomApps: []Application{}}
}
func (s Settings) MethodForPath(path string) Method {
	method := s.Default
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if s.JSON.OpenWith != "inherit" {
			method = s.JSON
		}
	case ".html", ".htm":
		if s.HTML.OpenWith != "inherit" {
			method = s.HTML
		}
	}
	return method
}
func builtinApplication(path string) string {
	switch strings.ToLower(filepath.Base(filepath.Clean(path))) {
	case "finder.app":
		return "finder"
	case "textedit.app":
		return "textedit"
	case "visual studio code.app":
		return "vscode"
	}
	return ""
}

type Store interface {
	ReadFileSettings(context.Context) (Settings, error)
	WriteFileSettings(context.Context, Settings) error
}
type Service struct {
	store    Store
	root     string
	platform string
	run      func(context.Context, string, ...string) ([]byte, error)
	picker   sync.Mutex
}

func NewService(store Store, root string) *Service {
	return &Service{store: store, root: root, platform: runtime.GOOS, run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, name, args...).Output()
	}}
}
func OpenURL(path string) string {
	if path == "" {
		return ""
	}
	return "/api/v1/files/open?" + url.Values{"path": {path}}.Encode()
}
func (s *Service) view(value Settings) View {
	v := View{Settings: value, Supported: s.platform == "darwin"}
	if value.CustomApps == nil {
		v.CustomApps = []Application{}
	}
	return v
}
func (s *Service) Get(ctx context.Context) (View, error) {
	value, err := s.store.ReadFileSettings(ctx)
	return s.view(value), err
}
func (s *Service) Save(ctx context.Context, edit Settings) (View, error) {
	if edit.Revision < 0 {
		return View{}, ErrInvalid
	}
	current, err := s.store.ReadFileSettings(ctx)
	if err != nil {
		return View{}, err
	}
	if edit.Revision != current.Revision {
		return View{}, ErrConflict
	}
	known := make(map[string]bool)
	for _, app := range current.CustomApps {
		known[app.Path] = true
	}
	seen := make(map[string]bool)
	apps := make([]Application, 0, len(edit.CustomApps))
	for _, app := range edit.CustomApps {
		path := app.Path
		// Keep registered apps removable even when they have been uninstalled.
		if !known[path] {
			path, err = applicationPath(path)
			if err != nil {
				return View{}, err
			}
		}
		if builtinApplication(path) != "" || seen[path] {
			continue
		}
		seen[path] = true
		apps = append(apps, Application{Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Path: path})
	}
	edit.CustomApps = apps
	// Validate all three methods together before persisting any changes.
	methods := []struct {
		next     *Method
		previous Method
		inherit  bool
	}{
		{&edit.Default, current.Default, false}, {&edit.JSON, current.JSON, true}, {&edit.HTML, current.HTML, true},
	}
	for _, item := range methods {
		method := item.next
		switch method.OpenWith {
		case "system", "vscode", "textedit", "finder":
			if method.CustomAppPath != "" {
				return View{}, ErrInvalid
			}
		case "inherit":
			if !item.inherit || method.CustomAppPath != "" {
				return View{}, ErrInvalid
			}
		case "custom":
			if preset := builtinApplication(method.CustomAppPath); preset != "" {
				// A picked preset is represented by its fixed choice, never a deletable copy.
				if _, err := applicationPath(method.CustomAppPath); err != nil {
					return View{}, err
				}
				*method = Method{OpenWith: preset}
			} else if !seen[method.CustomAppPath] {
				if item.previous.OpenWith != "custom" || item.previous.CustomAppPath != method.CustomAppPath || !known[method.CustomAppPath] {
					return View{}, ErrInvalid
				}
				if item.inherit {
					*method = Method{OpenWith: "inherit"}
				} else {
					*method = Method{OpenWith: "system"}
				}
			}
		default:
			return View{}, ErrInvalid
		}
	}
	if err := s.store.WriteFileSettings(ctx, edit); err != nil {
		return View{}, err
	}
	edit.Revision++
	return s.view(edit), nil
}
func applicationPath(path string) (string, error) {
	if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Ext(filepath.Clean(path)), ".app") {
		return "", ErrApplication
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", ErrApplication
	}
	info, err := os.Stat(filepath.Join(resolved, "Contents", "Info.plist"))
	if err != nil || !info.Mode().IsRegular() {
		return "", ErrApplication
	}
	return resolved, nil
}
func (s *Service) PickApplication(ctx context.Context) (*Application, error) {
	if s.platform != "darwin" {
		return nil, ErrUnsupported
	}
	if !s.picker.TryLock() {
		return nil, ErrPickerBusy
	}
	defer s.picker.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	// Browse application bundles directly, starting in /Applications, rather than
	// using the system's application list. Keep bundles selectable as single files.
	// Fixed script: neither a file path nor user text is interpolated into AppleScript.
	script := `try
 return POSIX path of (choose file with prompt "选择用于打开文件的应用" of type {"com.apple.application-bundle"} default location (POSIX file "/Applications/" as alias) without multiple selections allowed and showing package contents)
 on error number -128
 return ""
 end try`
	output, err := s.run(ctx, "/usr/bin/osascript", "-e", script)
	if err != nil {
		return nil, errors.New("无法完成应用选择，请重试；如有系统权限提示，请允许后再选择")
	}
	path := strings.TrimSpace(string(output))
	if path == "" {
		return nil, nil
	}
	path, err = applicationPath(path)
	if err != nil {
		return nil, err
	}
	return &Application{Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Path: path, Builtin: builtinApplication(path)}, nil
}
func (s *Service) Open(ctx context.Context, path string) error {
	if s.platform != "darwin" {
		return ErrUnsupported
	}
	// Resolve symlinks before checking containment; URL and relative paths are not executable inputs.
	if !filepath.IsAbs(path) || strings.ContainsRune(path, '\x00') {
		return ErrPath
	}
	root, err := filepath.EvalSymlinks(s.root)
	if err != nil {
		return ErrPath
	}
	resolved, err := filepath.EvalSymlinks(path)
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return ErrPath
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return ErrPath
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return ErrNotFound
	}
	if !info.Mode().IsRegular() {
		return ErrPath
	}
	settings, err := s.store.ReadFileSettings(ctx)
	if err != nil {
		return errors.New("文件设置读取失败，请到设置中刷新后重试")
	}
	method := settings.MethodForPath(path)
	args := []string{}
	switch method.OpenWith {
	case "system":
	case "vscode":
		args = append(args, "-b", "com.microsoft.VSCode")
	case "textedit":
		args = append(args, "-e")
	case "finder":
		args = append(args, "-R")
	case "custom":
		app, err := applicationPath(method.CustomAppPath)
		if err != nil {
			return err
		}
		args = append(args, "-a", app)
	default:
		return ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := s.run(ctx, "/usr/bin/open", append(args, resolved)...); err != nil {
		names := map[string]string{"system": "系统默认应用", "vscode": "VS Code", "textedit": "文本编辑", "finder": "访达", "custom": "所选应用"}
		return fmt.Errorf("无法使用%s打开文件，请确认应用已安装且支持此文件，或在设置中更换打开方式", names[method.OpenWith])
	}
	return nil
}
