package sqlite

import (
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/store"
)

func nowUnix() int64 { return time.Now().Unix() }

func mapProjectWriteErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case strings.Contains(err.Error(), "GIT_COMMIT_ACTIVE"):
		return store.ErrGitCommitActive
	case strings.Contains(err.Error(), "RUN_ACTIVE"):
		return store.ErrRunActive
	default:
		return err
	}
}
