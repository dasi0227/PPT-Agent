package sessionstate

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

type ToolRecoveryPolicy string

const (
	ToolReadOnly   ToolRecoveryPolicy = "read_only"
	ToolIdempotent ToolRecoveryPolicy = "idempotent"
	ToolReconcile  ToolRecoveryPolicy = "reconcile"
	ToolUnknown    ToolRecoveryPolicy = "unknown"
)

// A logical call can have several attempts, but only one committed native
// result. Bodies are private; the public projection is committed with them.
type ToolExecution struct {
	ID              string
	RunID           string
	CallID          string
	AttemptNo       int
	Name            string
	Arguments       json.RawMessage
	ArgumentsHash   string
	Status          string
	RecoveryPolicy  ToolRecoveryPolicy
	OperationID     string
	ResultJSON      json.RawMessage
	ResultMessageID string
	EntityRevision  int64
}

type ToolExecutionReader interface {
	GetToolExecution(context.Context, string, string) (ToolExecution, error)
}

func ToolAttemptID(runID, callID string, attempt int) string {
	return fmt.Sprintf("tool_%x", sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d", runID, callID, attempt))))
}

func ToolArgumentsHash(name string, args map[string]any) (string, json.RawMessage, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("%x", sha256.Sum256(append([]byte(name+"\x00"), raw...))), raw, nil
}

func (t ToolExecution) Call() (llm.ToolCall, error) {
	call := llm.ToolCall{ID: t.CallID, Name: t.Name}
	if err := json.Unmarshal(t.Arguments, &call.Args); err != nil {
		return call, err
	}
	hash, _, err := ToolArgumentsHash(call.Name, call.Args)
	if err != nil {
		return call, err
	}
	if t.CallID == "" || t.Name == "" || t.ArgumentsHash != hash {
		return call, ErrInvalidState
	}
	return call, nil
}

func (t ToolExecution) Terminal() bool {
	return t.Status == "succeeded" || t.Status == "failed" || t.Status == "canceled"
}
