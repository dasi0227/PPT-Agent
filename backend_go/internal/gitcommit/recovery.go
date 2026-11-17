package gitcommit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

type CommitIntent struct {
	AttemptID     string  `json:"attempt_id"`
	OriginalHEAD  string  `json:"original_head"`
	Tree          string  `json:"tree"`
	MessageDigest string  `json:"message_digest"`
	Message       Message `json:"message"`
}

func commitText(message Message) string {
	text := message.Title
	if len(message.Items) > 0 {
		items := make([]string, len(message.Items))
		for i, item := range message.Items {
			items[i] = "- " + item
		}
		text += "\n\n" + strings.Join(items, "\n")
	}
	return strings.TrimSpace(text)
}
func digestMessage(text string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(text)))
	return hex.EncodeToString(hash[:])
}
func (e *Executor) PrepareIntent(ctx context.Context, workDir, attemptID string, changes ChangeSet, message Message) (CommitIntent, error) {
	head, err := e.run(ctx, workDir, nil, "rev-parse", "--verify", "HEAD")
	if err != nil {
		count, countErr := e.run(ctx, workDir, nil, "rev-list", "--all", "--count")
		if countErr != nil || strings.TrimSpace(count) != "0" {
			return CommitIntent{}, err
		}
		head = ""
	}
	tree, err := e.run(ctx, workDir, []string{"GIT_INDEX_FILE=" + changes.IndexPath}, "write-tree")
	if err != nil {
		return CommitIntent{}, err
	}
	return CommitIntent{AttemptID: attemptID, OriginalHEAD: strings.TrimSpace(head), Tree: strings.TrimSpace(tree), MessageDigest: digestMessage(commitText(message)), Message: message}, nil
}

// Reconcile only observes Git. A missing or ambiguous marker never retries commit.
func (e *Executor) Reconcile(ctx context.Context, workDir string, intent CommitIntent) (Result, bool, error) {
	log, err := e.run(ctx, workDir, nil, "reflog", "show", "--format=%H%x00%gs", "HEAD")
	if err != nil {
		return Result{}, false, err
	}
	hash := ""
	marker := "ppt-agent-attempt-" + intent.AttemptID
	for _, line := range strings.Split(log, "\n") {
		parts := strings.SplitN(line, "\x00", 2)
		if len(parts) != 2 || !strings.HasPrefix(parts[1], marker+":") {
			continue
		}
		if hash != "" && hash != parts[0] {
			return Result{}, false, errors.New("ambiguous commit attempt marker")
		}
		hash = parts[0]
	}
	if hash == "" {
		return Result{}, false, nil
	}
	raw, err := e.run(ctx, workDir, nil, "show", "-s", "--format=%T%x00%P%x00%B", hash)
	if err != nil {
		return Result{}, false, err
	}
	parts := strings.SplitN(raw, "\x00", 3)
	if len(parts) != 3 || parts[0] != intent.Tree || parts[1] != intent.OriginalHEAD || digestMessage(parts[2]) != intent.MessageDigest {
		return Result{}, false, errors.New("commit attempt marker does not match its durable intent")
	}
	branch, err := e.run(ctx, workDir, nil, "branch", "--show-current")
	if err != nil {
		return Result{}, false, err
	}
	date, err := e.run(ctx, workDir, nil, "show", "-s", "--format=%cI", hash)
	if err != nil {
		return Result{}, false, err
	}
	stat, err := e.run(ctx, workDir, nil, "show", "--format=", "--numstat", "--no-renames", hash)
	if err != nil {
		return Result{}, false, err
	}
	files, insertions, deletions := parseNumStat(stat)
	return Result{Hash: hash[:7], Branch: strings.TrimSpace(branch), CommittedAt: strings.TrimSpace(date), FilesChanged: files, Insertions: insertions, Deletions: deletions}, true, nil
}
