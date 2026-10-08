package service_test

import (
	"encoding/json"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/service"
	"github.com/dasi0227/PPT-Agent/backend/internal/threadjournal"
)

func TestNamingCommandsHaveNoPublicTimelineProjection(t *testing.T) {
	for _, source := range []string{"user", "automatic"} {
		for _, status := range []string{"accepted", "running", "completed", "failed", "canceled"} {
			raw, _ := json.Marshal(map[string]any{"kind": "rename", "source": source, "status": status})
			_, visible, err := service.PublicThreadEvent(threadjournal.Event{Type: "command." + status, Payload: raw})
			if err != nil || visible {
				t.Fatalf("rename leaked into timeline: %s %s", source, status)
			}
		}
	}
}
