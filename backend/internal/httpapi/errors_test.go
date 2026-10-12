package httpapi

import (
	"errors"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestAPIErrorProjectionUsesSafeMessageAndWhitelistedDetails(t *testing.T) {
	agentErr := model.NewAgentError("COMPONENT_DISABLED", "resolve_components", errors.New(
		"/Users/private/slide.html database error api_key=secret stack trace",
	))
	agentErr.Details = map[string]any{
		"component_name":  "comparison-table",
		"raw_html":        "<html>secret</html>",
		"provider_result": "reasoning",
	}
	projected := ProjectAgentError(agentErr, "INTERNAL", "resolve_components")
	if projected.Code != "COMPONENT_DISABLED" || projected.HTTPStatus != 409 || projected.Retryable {
		t.Fatalf("API projection mismatch: %+v", projected)
	}
	if len(projected.Details) != 1 || projected.Details["component_name"] != "comparison-table" {
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
