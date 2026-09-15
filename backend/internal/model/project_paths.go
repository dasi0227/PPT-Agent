package model

import "path/filepath"

// ProjectRoot is the private project container; WorkDir is always its artifacts directory.
func ProjectRoot(workDir string) string { return filepath.Dir(filepath.Clean(workDir)) }

func UserHistoryPath(threadID string) string {
	return filepath.ToSlash(filepath.Join("threads", threadID, "user.jsonl"))
}

func ModelHistoryPath(threadID string) string {
	return filepath.ToSlash(filepath.Join("threads", threadID, "model.jsonl"))
}
