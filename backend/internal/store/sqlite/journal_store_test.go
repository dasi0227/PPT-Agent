package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newJournalTestStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	db, closeDB, err := Open(&config.Config{WorkRoot: root, DBPath: filepath.Join(root, "db", "ppt.db")}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeDB)
	if err := initializeSchema(db); err != nil {
		t.Fatal(err)
	}
	st := &Store{db: db, log: zap.NewNop(), workRoot: root}
	if err := db.Exec("INSERT INTO projects(id,title,theme,created_at,updated_at) VALUES('p','Project','theme',1,1)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO threads(id,project_id,title,created_at,updated_at) VALUES('t','p','Thread',1,1)").Error; err != nil {
		t.Fatal(err)
	}
	return st
}

func TestCurrentSchemaInitializationAndOldFormatRejection(t *testing.T) {
	st := newJournalTestStore(t)
	if err := initializeSchema(st.db); err != nil {
		t.Fatal("repeat initialization", err)
	}
	var title string
	if err := st.db.Raw("SELECT title FROM projects WHERE id='p'").Scan(&title).Error; err != nil {
		t.Fatal(err)
	}
	if title != "Project" {
		t.Fatal("initialization replaced existing state")
	}
	if err := st.db.Exec("PRAGMA user_version=0").Error; err != nil {
		t.Fatal(err)
	}
	if err := initializeSchema(st.db); err == nil {
		t.Fatal("old format was accepted")
	}
	if err := st.db.Raw("SELECT title FROM projects WHERE id='p'").Scan(&title).Error; err != nil {
		t.Fatal("refusal destroyed data", err)
	}
}

func TestOutboxReconcilesCrashAfterLogWrite(t *testing.T) {
	st := newJournalTestStore(t)
	ctx := context.Background()
	if err := st.db.Exec("CREATE TRIGGER fail_delivery BEFORE DELETE ON thread_event_outbox BEGIN SELECT RAISE(ABORT,'injected database failure'); END").Error; err != nil {
		t.Fatal(err)
	}
	event := threadjournal.Event{Type: "message", Payload: json.RawMessage(`{"text":"once"}`)}
	if _, err := st.AppendThreadEvent(ctx, "t", event); err == nil {
		t.Fatal("delivery failure concealed")
	}
	before, err := threadjournal.Read(st.projectRoot("p"), "p", "t")
	if err != nil || len(before) != 1 {
		t.Fatal("file did not precede receipt", err)
	}
	if err := st.db.Exec("DROP TRIGGER fail_delivery").Error; err != nil {
		t.Fatal(err)
	}
	if err := st.recoverOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := st.ThreadEvents(ctx, "t", 0)
	if err != nil || len(after) != 1 {
		t.Fatalf("replay duplicated history: %v %v", after, err)
	}
	if before[0].ID != after[0].ID {
		t.Fatal("retry changed event identity")
	}
	var pending int64
	if err := st.db.Model(&outboxPO{}).Count(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if pending != 0 {
		t.Fatal("delivered outbox was not cleaned")
	}
}

func TestOutboxRetainsAcceptedEventOnFileFailure(t *testing.T) {
	st := newJournalTestStore(t)
	ctx := context.Background()
	path, err := threadjournal.Path(st.projectRoot("p"), "t")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	event, err := st.AppendThreadEvent(ctx, "t", threadjournal.Event{Type: "message", Payload: json.RawMessage(`{"text":"accepted"}`)})
	if err == nil || event.Seq != 1 {
		t.Fatalf("accepted event or failure lost: %+v %v", event, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := st.recoverOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	events, err := st.ThreadEvents(ctx, "t", 0)
	if err != nil || len(events) != 1 || events[0].ID != event.ID {
		t.Fatalf("accepted event not recovered: %+v %v", events, err)
	}
}

func TestBusinessStateAndEventIntentRollbackTogether(t *testing.T) {
	st := newJournalTestStore(t)
	ctx := context.Background()
	injected := errors.New("transaction failed")
	err := st.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("UPDATE threads SET title='Changed' WHERE id='t'").Error; err != nil {
			return err
		}
		if _, err := st.enqueueEvent(tx, "t", threadjournal.Event{Type: "renamed", Payload: json.RawMessage(`{"title":"Changed"}`)}); err != nil {
			return err
		}
		return injected
	})
	if !errors.Is(err, injected) {
		t.Fatal(err)
	}
	var row struct {
		Title        string
		NextEventSeq int64
	}
	if err := st.db.Table("threads").Where("id='t'").Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Title != "Thread" || row.NextEventSeq != 1 {
		t.Fatal("rolled-back state leaked", row)
	}
	events, err := st.ThreadEvents(ctx, "t", 0)
	if err != nil || len(events) != 0 {
		t.Fatal("rolled-back event became visible", err)
	}
}
