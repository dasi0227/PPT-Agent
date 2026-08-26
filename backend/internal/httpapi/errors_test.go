package httpapi

import (
	"errors"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestAPIErrorProjectionUsesSafeMessageAndWhitelistedDetails(t *testing.T) {
	agentErr := model.NewAgentError("REVISION_CONFLICT", "mutate_ppt", errors.New(
		"/Users/private/slide.html database error api_key=secret stack trace",
	))
	agentErr.Details = map[string]any{
		"current_revision": 9,
		"next_action":      "read the current resource",
		"raw_html":         "<html>secret</html>",
		"provider_result":  "reasoning",
	}
	projected := ProjectAgentError(agentErr, "INTERNAL", "mutate_ppt")
	if projected.Code != "REVISION_CONFLICT" || projected.HTTPStatus != 409 || projected.Retryable {
		t.Fatalf("API projection mismatch: %+v", projected)
	}
	if projected.Details["current_revision"] != 9 || projected.Details["next_action"] == "" ||
		projected.Details["raw_html"] != nil || projected.Details["provider_result"] != nil {
		t.Fatalf("unsafe API details projection: %+v", projected.Details)
	}
	publicText := projected.Message
	for _, forbidden := range []string{"/Users/", "database error", "api_key", "stack trace", "<html>", "reasoning"} {
		if strings.Contains(publicText, forbidden) {
			t.Fatalf("API message leaked %q: %s", forbidden, publicText)
		}
	}
}

func TestProviderUnavailableAPIProjectionIsRetryableAndSafe(t *testing.T) {
	agentErr := model.NewAgentError(
		"PROVIDER_UNAVAILABLE",
		"provider_request",
		errors.New("POST https://provider.example body=upstream-secret Authorization=Bearer sk-secret"),
	)
	projected := ProjectAgentError(agentErr, "INTERNAL", "provider_request")
	if projected.Code != "PROVIDER_UNAVAILABLE" || projected.HTTPStatus != 503 || !projected.Retryable {
		t.Fatalf("provider API projection mismatch: %+v", projected)
	}
	for _, forbidden := range []string{"provider.example", "upstream-secret", "Authorization", "sk-secret"} {
		if strings.Contains(projected.Message, forbidden) {
			t.Fatalf("API message leaked %q: %s", forbidden, projected.Message)
		}
	}
}
