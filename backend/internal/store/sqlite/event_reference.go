package sqlite

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
	"gorm.io/gorm"
)

type runAcceptance struct {
	Command         model.RunCommand `json:"command"`
	ClientRequestID string           `json:"client_request_id"`
}

func workRootFromDB(db *gorm.DB) (string, error) {
	value, _ := db.Get("ppt.work_root")
	root, _ := value.(string)
	if root == "" {
		return "", errors.New("store requires a work directory")
	}
	return root, nil
}

// Reads references without acquiring another connection or flushing inside a
// business transaction. Unpublished events can only be read from its outbox.
func eventInTransaction(tx *gorm.DB, threadID string, seq int64) (threadjournal.Event, error) {
	db := tx.Session(&gorm.Session{NewDB: true})
	root, err := workRootFromDB(tx)
	if err != nil {
		return threadjournal.Event{}, err
	}
	var thread journalThreadPO
	if err := db.First(&thread, "id = ?", threadID).Error; err != nil {
		return threadjournal.Event{}, mapErr(err)
	}
	projectRoot := filepath.Join(root, "projects", thread.ProjectID)
	var row outboxPO
	err = db.First(&row, "thread_id = ? AND seq = ?", threadID, seq).Error
	if err == nil {
		var e threadjournal.Event
		if err := json.Unmarshal([]byte(row.EventJSON), &e); err != nil {
			return e, err
		}
		return threadjournal.Resolve(projectRoot, threadID, e)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return threadjournal.Event{}, err
	}
	events, err := threadjournal.Read(projectRoot, thread.ProjectID, threadID)
	if err != nil {
		return threadjournal.Event{}, err
	}
	if seq < 1 || seq > int64(len(events)) {
		return threadjournal.Event{}, fmt.Errorf("%w: event reference %d", threadjournal.ErrCorrupt, seq)
	}
	return events[seq-1], nil
}
