package model

import (
	"errors"
	"strings"
	"testing"
)

func TestAgentErrorRegistryAndRetryAuthority(t *testing.T) {
	seen := map[string]bool{}
	for _, code := range RegisteredErrorCodes() {
		if seen[code] {
			t.Fatalf("duplicate registered error code %q", code)
		}
		seen[code] = true
		def := ErrorDefinitionFor(code)
		if def.Code != code || def.Category == "" || def.SafeMessage == "" || def.ModelMessage == "" {
			t.Fatalf("incomplete definition for %q: %+v", code, def)
		}
		if def.Retryable != (def.Category == ErrorTransient) {
			t.Fatalf("only transient errors may be retryable: %+v", def)
		}
	}
	for _, code := range []string{
		"CONTENT_INVALID", "EDIT_ANCHOR_NOT_FOUND", "REVISION_CONFLICT",
		"IDEMPOTENCY_KEY_REUSED", "RUN_CANCELED", "COMMIT_FAILED",
	} {
		if NewAgentError(code, "test", nil).ShouldAutoRetry() {
			t.Fatalf("%s must not be automatically retried", code)
		}
	}
	if !NewAgentError("PROVIDER_UNAVAILABLE", "provider", nil).ShouldAutoRetry() {
		t.Fatal("transient provider error should be automatically retryable")
	}
}

func TestAgentErrorProjectionsKeepPublicPayloadSafe(t *testing.T) {
	secret := "/Users/private/project/slide.html api_key=sk-secret raw=<html> stack trace database error provider reasoning"
	agentErr := NewAgentError("CONTENT_INVALID", "write_ppt", errors.New(secret))
	agentErr.CallID = "call-7"
	agentErr.Resource = &ErrorResource{Type: "slide", SlideID: "slide-2", Part: "html"}
	agentErr.Details = map[string]any{
		"json_pointer": "/slides/1/title",
		"html_checks":  []string{"root dimensions", "overflow"},
		"revision":     12,
		"next_action":  "correct the invalid content and call write_ppt again",
	}

	modelView := agentErr.ModelObservation()
	if modelView["json_pointer"] != "/slides/1/title" || modelView["call_id"] != "call-7" ||
		modelView["revision"] != 12 {
		t.Fatalf("model projection lacks actionable repair details: %+v", modelView)
	}
	publicView := agentErr.Public()
	publicText := publicView.Code + " " + publicView.Message
	for _, forbidden := range []string{"/Users/", "sk-secret", "<html>", "stack trace", "database error", "provider reasoning"} {
		if strings.Contains(publicText, forbidden) {
			t.Fatalf("public error leaked %q: %+v", forbidden, publicView)
		}
	}
	trace := agentErr.TraceProjection()
	if trace["cause"] != secret || agentErr.HTTPStatus() != 422 {
		t.Fatalf("trace/http projection mismatch: trace=%+v status=%d", trace, agentErr.HTTPStatus())
	}
}
