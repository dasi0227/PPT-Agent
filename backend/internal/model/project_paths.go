package model

import "path/filepath"

// ProjectRoot is the private project container; WorkDir is always its artifacts directory.
func ProjectRoot(workDir string) string { return filepath.Dir(filepath.Clean(workDir)) }

func ThreadJournalPath(threadID string) string {
	return filepath.ToSlash(filepath.Join("threads", threadID, "thread.jsonl"))
}
