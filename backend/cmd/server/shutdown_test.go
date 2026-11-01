package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/run"
	"go.uber.org/zap"
)

func TestShutdownEndsOpenStreamAndSharesDeadline(t *testing.T) {
	requestCtx, cancelRequests := context.WithCancel(context.Background())
	defer cancelRequests()
	stream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: ready\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	stream.Config.BaseContext = func(net.Listener) context.Context { return requestCtx }
	stream.Start()
	defer stream.Close()
	response, err := stream.Client().Get(stream.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	closedExports := make(chan context.Context, 1)
	app := &App{server: stream.Config, engine: run.NewEngine(nil, nil, zap.NewNop()), log: zap.NewNop(),
		stopBackground:   func() {},
		shutdownCommands: func(context.Context) error { return nil },
		shutdownExports:  func(ctx context.Context) error { closedExports <- ctx; return nil },
		shutdownRenderer: func(rendererCtx context.Context) error {
			if rendererCtx != ctx || <-closedExports != ctx {
				t.Error("shutdown phases did not share the deadline")
			}
			return nil
		},
	}
	if err := app.shutdown(ctx, cancelRequests); err != nil {
		t.Fatalf("open stream prevented shutdown: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatal("idle shutdown exhausted its deadline")
	}
}
