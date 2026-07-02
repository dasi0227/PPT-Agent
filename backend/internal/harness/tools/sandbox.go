package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Sandbox 把所有文件读写限定在 root（project work_dir 或全局 _assets）内（ARCH-TOOLS-006）。
// 路径规范化后做前缀校验，拒绝 ..、绝对路径逃逸与符号链接逃逸。
type Sandbox struct {
	root string
}

func NewSandbox(root string) (*Sandbox, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	// 解析 root 自身的符号链接，得到权威根。root 不存在时按字面绝对路径处理。
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return &Sandbox{root: abs}, nil
}

// Resolve 校验相对路径并返回其绝对路径；越界返回错误，调用方 MUST 整体失败不写文件。
func (s *Sandbox) Resolve(rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escapes work_dir: absolute path %q not allowed", rel)
	}
	joined := filepath.Join(s.root, rel)
	clean := filepath.Clean(joined)
	// 对已存在的父链解析符号链接，防止软链逃逸。
	resolved := clean
	if r, err := filepath.EvalSymlinks(clean); err == nil {
		resolved = r
	} else if r, err := filepath.EvalSymlinks(filepath.Dir(clean)); err == nil {
		resolved = filepath.Join(r, filepath.Base(clean))
	}
	if !within(s.root, resolved) {
		return "", fmt.Errorf("path escapes work_dir: %q", rel)
	}
	return clean, nil
}

func (s *Sandbox) Read(rel string) ([]byte, error) {
	abs, err := s.Resolve(rel)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(abs)
}

func (s *Sandbox) Write(rel string, data []byte) error {
	abs, err := s.Resolve(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, data, 0o644)
}

func (s *Sandbox) Delete(rel string) error {
	abs, err := s.Resolve(rel)
	if err != nil {
		return err
	}
	return os.Remove(abs)
}

func within(root, target string) bool {
	if target == root {
		return true
	}
	return strings.HasPrefix(target, root+string(os.PathSeparator))
}
