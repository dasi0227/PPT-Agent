package sourceformat

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

//go:embed html.mjs
var htmlScript string

// HTML runs the same formatter for every durable write, without executing HTML.
// Reuse the rendering runtime's Node installation and dependency directory.
func HTML(ctx context.Context, raw []byte) ([]byte, error) {
	directory, err := formatterDirectory()
	if err != nil {
		return nil, err
	}
	node := os.Getenv("PPT_RENDER_NODE")
	if node == "" {
		node = "node"
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, "--input-type=module", "-e", htmlScript)
	cmd.Dir = directory
	cmd.Stdin = bytes.NewReader(raw)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	formatted, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("HTML formatting failed: %w: %s", err, stderr.String())
	}
	return formatted, nil
}
func formatterDirectory() (string, error) {
	candidates := []string{}
	if worker := os.Getenv("PPT_RENDER_WORKER"); worker != "" {
		candidates = append(candidates, filepath.Dir(worker))
	}
	candidates = append(candidates, "render-worker", filepath.Join("backend", "render-worker"))
	// Source location is useful when invoked from an internal package in development.
	if _, source, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(source), "..", "..", "render-worker"))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(filepath.Join(candidate, "package.json")); err == nil && !info.IsDir() {
			return filepath.Abs(candidate)
		}
	}
	return "", fmt.Errorf("HTML formatter dependency directory is unavailable")
}
func JSON(raw []byte) ([]byte, error) {
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, bytes.TrimSpace(raw), "", "  "); err != nil {
		return nil, err
	}
	formatted.WriteByte('\n')
	return formatted.Bytes(), nil
}
