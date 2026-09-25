// Package threadjournal owns the durable, ordered facts of a conversation.
// SQLite allocates identities and sequence numbers; this package only delivers
// and verifies those records. Replaying a journal never executes an action.
package threadjournal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const Version = 1
const InlineLimit = 64 << 10
const MaxRecordBytes = 16 << 20

var ErrCorrupt = errors.New("conversation journal is corrupt")

type Event struct {
	PayloadFields map[string]string `json:"payload_fields,omitempty"`
	ID            string            `json:"id"`
	Seq           int64             `json:"seq"`
	TS            int64             `json:"ts"`
	Type          string            `json:"type"`
	RunID         string            `json:"run_id,omitempty"`
	CommandID     string            `json:"command_id,omitempty"`
	AttemptID     string            `json:"attempt_id,omitempty"`
	Payload       json.RawMessage   `json:"payload,omitempty"`
	PayloadRef    string            `json:"payload_ref,omitempty"`
}

type Header struct {
	Format    string `json:"format"`
	Version   int    `json:"version"`
	ProjectID string `json:"project_id"`
	ThreadID  string `json:"thread_id"`
}

type Backend interface {
	AppendThreadEvent(context.Context, string, Event) (Event, error)
	ThreadEvents(context.Context, string, int64) ([]Event, error)
	FlushThreadEvents(context.Context, string) error
}

var pathLocks sync.Map

func lock(path string) func() {
	value, _ := pathLocks.LoadOrStore(filepath.Clean(path), &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func validID(id string) bool {
	return id != "" && id != "." && id != ".." && filepath.Base(id) == id && !strings.ContainsAny(id, "/\\\x00")
}

func Path(projectRoot, threadID string) (string, error) {
	if !validID(threadID) {
		return "", errors.New("invalid thread identity")
	}
	return filepath.Join(projectRoot, "threads", threadID, "thread.jsonl"), nil
}

func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

// Sync every newly linked directory, including its entry in the parent. A file
// fsync alone does not make a newly created threads/<id> path crash durable.
func makeDurableDir(path string) error {
	var missing []string
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Stat(current)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("journal parent is not a directory: %s", current)
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		missing = append(missing, current)
		if filepath.Dir(current) == current {
			return err
		}
	}
	for index := len(missing) - 1; index >= 0; index-- {
		if err := os.Mkdir(missing[index], 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		if err := syncDir(missing[index]); err != nil {
			return err
		}
		if err := syncDir(filepath.Dir(missing[index])); err != nil {
			return err
		}
	}
	return nil
}

// Publish is content addressed. The reference is relative and contains no user path.
func Publish(projectRoot, threadID string, payload json.RawMessage) (string, error) {
	if !json.Valid(payload) {
		return "", errors.New("invalid event payload")
	}
	path, err := Path(projectRoot, threadID)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	ref := hex.EncodeToString(sum[:])
	dir := filepath.Join(filepath.Dir(path), "payloads")
	if err := makeDurableDir(dir); err != nil {
		return "", err
	}
	target := filepath.Join(dir, ref+".json")
	if existing, err := os.ReadFile(target); err == nil {
		if !bytes.Equal(existing, payload) {
			return "", fmt.Errorf("%w: payload collision", ErrCorrupt)
		}
		return ref, syncDir(dir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".payload-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(payload); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	if err = os.Rename(tmp.Name(), target); err != nil {
		return "", err
	}
	return ref, syncDir(dir)
}

func readPayload(path, ref string) (json.RawMessage, error) {
	decoded, err := hex.DecodeString(ref)
	if err != nil || len(decoded) != sha256.Size || len(ref) != 64 {
		return nil, fmt.Errorf("%w: payload identity", ErrCorrupt)
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(path), "payloads", ref+".json"))
	if err != nil {
		return nil, fmt.Errorf("journal payload %s: %w", ref, err)
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != ref || !json.Valid(raw) {
		return nil, fmt.Errorf("%w: payload checksum", ErrCorrupt)
	}
	return raw, nil
}
func hydrate(path string, event *Event) error {
	if event.PayloadRef != "" {
		raw, err := readPayload(path, event.PayloadRef)
		if err != nil {
			return err
		}
		event.Payload = raw
	}
	return hydrateFields(path, event)
}

func validate(event Event) error {
	if event.ID == "" || event.Seq < 1 || event.TS < 1 || event.Type == "" || (len(event.Payload) == 0) == (event.PayloadRef == "") {
		return fmt.Errorf("%w: invalid event envelope", ErrCorrupt)
	}
	if len(event.Payload) > 0 && !json.Valid(event.Payload) {
		return fmt.Errorf("%w: invalid payload", ErrCorrupt)
	}
	return nil
}

// scan accepts only complete newline-terminated records. A writer can repair an
// incomplete final record; an invalid complete line is never silently skipped.
func scan(path string, expected Header) ([]Event, int64, bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []Event{}, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	end := bytes.LastIndexByte(raw, '\n') + 1
	if end == 0 {
		return nil, 0, len(raw) > 0, nil
	}
	lines := bytes.Split(raw[:end-1], []byte{'\n'})
	var header Header
	if json.Unmarshal(lines[0], &header) != nil || header != expected {
		return nil, 0, false, fmt.Errorf("%w: identity or format mismatch", ErrCorrupt)
	}
	events := make([]Event, 0, len(lines)-1)
	ids := map[string]bool{}
	for index, line := range lines[1:] {
		var event Event
		if len(line) > MaxRecordBytes || json.Unmarshal(line, &event) != nil {
			return nil, 0, false, fmt.Errorf("%w: line %d", ErrCorrupt, index+2)
		}
		if err := validate(event); err != nil {
			return nil, 0, false, err
		}
		if event.Seq != int64(index+1) || ids[event.ID] {
			return nil, 0, false, fmt.Errorf("%w: sequence or event identity", ErrCorrupt)
		}
		ids[event.ID] = true
		events = append(events, event)
	}
	return events, int64(end), end != len(raw), nil
}

func identity(projectID, threadID string) Header {
	return Header{Format: "ppt-agent-thread", Version: Version, ProjectID: projectID, ThreadID: threadID}
}

func Read(projectRoot, projectID, threadID string) ([]Event, error) {
	path, err := Path(projectRoot, threadID)
	if err != nil {
		return nil, err
	}
	unlock := lock(path)
	defer unlock()
	events, _, torn, err := scan(path, identity(projectID, threadID))
	if err != nil {
		return nil, err
	}
	if torn {
		return nil, fmt.Errorf("%w: incomplete tail requires delivery recovery", ErrCorrupt)
	}
	for index := range events {
		if err := hydrate(path, &events[index]); err != nil {
			return nil, err
		}
	}
	return events, nil
}

// Recover repairs only an uncommitted partial tail. It never recreates a
// database-acknowledged event whose bytes have disappeared from the journal.
func Recover(projectRoot, projectID, threadID string, delivered int64) error {
	path, err := Path(projectRoot, threadID)
	if err != nil {
		return err
	}
	unlock := lock(path)
	defer unlock()
	events, end, torn, err := scan(path, identity(projectID, threadID))
	if err != nil {
		return err
	}
	if int64(len(events)) < delivered {
		return fmt.Errorf("%w: acknowledged records are missing", ErrCorrupt)
	}
	for index := range events {
		if err := hydrate(path, &events[index]); err != nil {
			return err
		}
	}
	if !torn {
		return nil
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Truncate(end); err != nil {
		return err
	}
	return f.Sync()
}

// Resolve verifies an outbox payload before a transaction consumes it.
func Resolve(projectRoot, threadID string, event Event) (Event, error) {
	path, err := Path(projectRoot, threadID)
	if err != nil {
		return event, err
	}
	if err := validate(event); err != nil {
		return event, err
	}
	err = hydrate(path, &event)
	return event, err
}

// Deliver is idempotent for the exact same allocated event, including after a
// crash between fsync and clearing the database outbox.
func Deliver(projectRoot, projectID, threadID string, event Event) error {
	if err := validate(event); err != nil {
		return err
	}
	path, err := Path(projectRoot, threadID)
	if err != nil {
		return err
	}
	unlock := lock(path)
	defer unlock()
	if err := makeDurableDir(filepath.Dir(path)); err != nil {
		return err
	}
	expected := identity(projectID, threadID)
	events, end, torn, err := scan(path, expected)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if torn {
		if err := f.Truncate(end); err != nil {
			return err
		}
	}
	if event.Seq <= int64(len(events)) {
		old, _ := json.Marshal(events[event.Seq-1])
		next, _ := json.Marshal(event)
		if !bytes.Equal(old, next) {
			return fmt.Errorf("%w: conflicting redelivery", ErrCorrupt)
		}
		check := event
		if err := hydrate(path, &check); err != nil {
			return err
		}
		return f.Sync()
	}
	if event.Seq != int64(len(events)+1) {
		return fmt.Errorf("%w: delivery gap", ErrCorrupt)
	}
	// Verify referenced bytes before making an event visible.
	check := event
	if err := hydrate(path, &check); err != nil {
		return err
	}
	if _, err := f.Seek(end, io.SeekStart); err != nil {
		return err
	}
	encoder := json.NewEncoder(f)
	encoder.SetEscapeHTML(false)
	if end == 0 {
		if err := encoder.Encode(expected); err != nil {
			return err
		}
	}
	if err := encoder.Encode(event); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}
