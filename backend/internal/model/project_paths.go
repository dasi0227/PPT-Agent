package model

import "path/filepath"

// ProjectRoot is the private project container; WorkDir is always its artifacts directory.
func ProjectRoot(workDir string) string { return filepath.Dir(filepath.Clean(workDir)) }
