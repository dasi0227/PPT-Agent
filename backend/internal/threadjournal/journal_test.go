package threadjournal

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fact(seq int64) Event {
	return Event{ID: "event-" + string(rune('a'+seq)), Seq: seq, TS: 1700000000000 + seq, Type: "message", Payload: json.RawMessage(`{"text":"hello"}`)}
}

func TestDeliveryRecoveryDoesNotDuplicateFacts(t *testing.T) {
	root := t.TempDir()
	event := fact(1)
	if err := Deliver(root, "project", "thread", event); err != nil {
		t.Fatal(err)
	}
	// Simulate a crash after file fsync but before clearing the database outbox.
	if err := Deliver(root, "project", "thread", event); err != nil {
		t.Fatal(err)
	}
	path, _ := Path(root, "thread")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"id":"partial`); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := Recover(root, "project", "thread", 1); err != nil {
		t.Fatal(err)
	}
	if err := Deliver(root, "project", "thread", fact(2)); err != nil {
		t.Fatal(err)
	}
	events, err := Read(root, "project", "thread")
	if err != nil || len(events) != 2 {
		t.Fatalf("facts = %v, %v", events, err)
	}
	if string(events[0].Payload) != string(event.Payload) {
		t.Fatal("original payload changed")
	}
	conflict := event
	conflict.Payload = json.RawMessage(`{"text":"different"}`)
	if err := Deliver(root, "project", "thread", conflict); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("conflicting replay: %v", err)
	}
}

func TestCommittedCorruptionIsNeverTruncated(t *testing.T) {
	for _, suffix := range []string{"not-json\n", "{}\n"} {
		t.Run(suffix, func(t *testing.T) {
			root := t.TempDir()
			if err := Deliver(root, "p", "t", fact(1)); err != nil {
				t.Fatal(err)
			}
			path, _ := Path(root, "t")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			raw = append(raw, []byte(suffix)...)
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if err := Recover(root, "p", "t", 1); !errors.Is(err, ErrCorrupt) {
				t.Fatalf("corruption accepted: %v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(raw) {
				t.Fatal("complete malformed record was destroyed")
			}
		})
	}
	if err := Recover(t.TempDir(), "p", "t", 1); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("lost acknowledged event accepted: %v", err)
	}
}

func TestImmutablePayloadVerification(t *testing.T) {
	root := t.TempDir()
	payload := json.RawMessage(`{"result":"large result"}`)
	ref, err := Publish(root, "t", payload)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Publish(root, "t", payload)
	if err != nil || ref != again {
		t.Fatal("payload identity is unstable", err)
	}
	event := fact(1)
	event.Payload = nil
	event.PayloadRef = ref
	if err := Deliver(root, "p", "t", event); err != nil {
		t.Fatal(err)
	}
	events, err := Read(root, "p", "t")
	if err != nil || len(events) != 1 || string(events[0].Payload) != string(payload) {
		t.Fatal("payload did not resolve", err)
	}
	if err := os.Remove(filepath.Join(root, "threads", "t", "payloads", ref+".json")); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root, "p", "t"); err == nil {
		t.Fatal("missing immutable payload accepted")
	}
	if err := Deliver(root, "p", "t", event); err == nil {
		t.Fatal("redelivery concealed missing payload")
	}
}

func TestJournalIdentityAndSequence(t *testing.T) {
	root := t.TempDir()
	if err := Deliver(root, "p", "t", fact(2)); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("gap accepted: %v", err)
	}
	if err := Deliver(root, "p", "t", fact(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root, "other", "t"); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("wrong project accepted: %v", err)
	}
	for _, id := range []string{"", "..", "a/b", "a\\b"} {
		if _, err := Path(root, id); err == nil {
			t.Fatalf("unsafe identity accepted: %q", id)
		}
	}
}

func TestSharedPayloadTextIsVerifiedAndDeduplicated(t *testing.T) {
	root := t.TempDir()
	text := strings.Repeat("same durable content ", 40)
	for i := int64(1); i <= 2; i++ {
		event := fact(i)
		event.Payload, _ = json.Marshal(map[string]any{"text": text, "large_number": json.Number("9007199254740993")})
		prepared, err := Prepare(root, "t", event)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(prepared.Payload, []byte(text)) {
			t.Fatal("body repeated in event")
		}
		if err := Deliver(root, "p", "t", prepared); err != nil {
			t.Fatal(err)
		}
		if err := Deliver(root, "p", "t", prepared); err != nil {
			t.Fatal(err)
		}
	}
	path, _ := Path(root, "t")
	payloads, err := os.ReadDir(filepath.Join(filepath.Dir(path), "payloads"))
	if err != nil || len(payloads) != 1 {
		t.Fatalf("duplicate payloads: %d %v", len(payloads), err)
	}
	events, err := Read(root, "p", "t")
	if err != nil || len(events) != 2 || !bytes.Contains(events[0].Payload, []byte(text)) || !bytes.Contains(events[0].Payload, []byte("9007199254740993")) {
		t.Fatalf("hydration: %+v %v", events, err)
	}
	if err := os.Remove(filepath.Join(filepath.Dir(path), "payloads", payloads[0].Name())); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root, "p", "t"); err == nil {
		t.Fatal("missing text payload was ignored")
	}
}
