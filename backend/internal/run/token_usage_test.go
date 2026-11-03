package run

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
 "github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
	"github.com/dasi0227/PPT-Agent/backend/internal/modelusage"
	"go.uber.org/zap"
)

type failingUsageStore struct {
	*memStore
	attempts int
}

func (s *failingUsageStore) ApplyRunUpdate(ctx context.Context, runID string, update sessionstate.RunUpdate) (sessionstate.Receipt,error) {
	if update.Type == model.EventRunUsageUpdated {
		s.attempts++
		return sessionstate.Receipt{},errors.New("usage write unavailable")
	}
	return s.memStore.ApplyRunUpdate(ctx, runID, update)
}

type usageResponseTransport struct{}

func (usageResponseTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":10,"output_tokens":2}}`))}, nil
}

func TestUsageWriteFailureDoesNotPauseOrCancelExecution(t *testing.T) {
	store := &failingUsageStore{memStore: newMemStore()}
	run := model.Run{ID: "usage-run", Status: model.RunRunning}
	store.runs[run.ID] = run
	bus := NewBus(run, store)
	if err := bus.Emit(context.Background(), model.EventRunStarted, model.RunStartedPayload{
		PublicEventBase: model.NewPublicEventBase(run.ID), Scope: model.NewRunScope(model.ScopeAllPages), Mode: model.ModeChat, UserInput: "test",
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &active{run: run, bus: bus, cancel: cancel}
	engine := NewEngine(store, nil, zap.NewNop())
	request, _ := http.NewRequestWithContext(engine.withUsageRecorder(ctx, a), http.MethodPost, "http://model.test", nil)
	response, err := modelusage.NewTransport(usageResponseTransport{}, "provider", "model").RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	_, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || store.attempts != 1 || a.pauseRequested || ctx.Err() != nil || store.runs[run.ID].Status != model.RunRunning {
		t.Fatalf("accounting altered execution: read=%v close=%v attempts=%d paused=%v context=%v", readErr, closeErr, store.attempts, a.pauseRequested, ctx.Err())
	}
}
