package model

import (
	"errors"
	"strings"
	"testing"
)

func TestAgentErrorRegistryAndRetryAuthority(t *testing.T) {
	for code, def := range errorDefinitions {
		if def.Code != code || def.Category == "" || def.SafeMessage == "" || def.ModelMessage == "" {
			t.Fatalf("incomplete definition for %q: %+v", code, def)
		}
		if def.Retryable != (def.Category == ErrorTransient) {
			t.Fatalf("only transient errors may be retryable: %+v", def)
		}
	}
	if len(commonErrorCodes) != 15 {
		t.Fatalf("expected fifteen common tool errors, got %v", commonErrorCodes)
	}
	for _, code := range commonErrorCodes {
		if _, registered := errorDefinitions[code]; !registered || !IsCommonErrorCode(code) {
			t.Fatalf("common error is not registered: %s", code)
		}
	}
	if !IsCommonErrorCode(CodeEditAmbiguous) || !IsCommonErrorCode(CodeEditNotFound) {
		t.Fatal("text edit failures must belong to the common error set")
	}
	if IsCommonErrorCode("IMAGE_INVALID") || IsCommonErrorCode("RENDER_FAILED") {
		t.Fatal("tool business failures must not enter the common error set")
	}
	for _, code := range []string{
		CodeContentInvalid, CodeEditNotFound, CodeEditAmbiguous, "RUN_REVISION_CONFLICT",
		"IDEMPOTENCY_KEY_REUSED", "RUN_ALREADY_CANCELED", "PERSISTENCE_FAILED",
		"MODEL_PROFILE_NOT_FOUND", CodeNotFound, CodeContentConflict, CodeOutlineNotExist,
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
	secret := "/Users/private/project/slide.html api_key=sk-secret raw=<html> stack trace database error provider reasoning Authorization=Bearer auth-secret provider_body=provider-raw"
	agentErr := NewAgentError(CodeContentInvalid, "edit_spec", errors.New(secret))
	agentErr.Reason = "Invalid content at /slides/1/title."
	agentErr.Measure = "correct the invalid content and call edit_spec again"
	agentErr.Details = map[string]any{
		"status":       "untrusted status",
		"json_pointer": "/slides/1/title",
		"html_checks":  []string{"root dimensions", "overflow"},
		"revision":     12,
	}

	modelView := agentErr.ModelObservation()
	if modelView["status"] != "error" || modelView["json_pointer"] != "/slides/1/title" || modelView["category"] != ErrorRepairable || modelView["reason"] != agentErr.Reason || modelView["measure"] != agentErr.Measure ||
		modelView["revision"] != 12 {
		t.Fatalf("model projection lacks actionable repair details: %+v", modelView)
	}
	publicView := agentErr.Public()
	publicText := publicView.Code + " " + publicView.Message
	for _, forbidden := range []string{"/Users/", "sk-secret", "auth-secret", "provider-raw", "<html>", "stack trace", "database error", "provider reasoning"} {
		if strings.Contains(publicText, forbidden) {
			t.Fatalf("public error leaked %q: %+v", forbidden, publicView)
		}
	}
	trace := agentErr.TraceProjection()
	traceCause, _ := trace["cause"].(string)
	for _, forbidden := range []string{"sk-secret", "api_key", "auth-secret", "Authorization", "provider-raw", "provider_body"} {
		if strings.Contains(traceCause, forbidden) {
			t.Fatalf("trace cause leaked %q: %s", forbidden, traceCause)
		}
	}
	if trace["operation"] != "edit_spec" || trace["retryable"] != false || !strings.Contains(traceCause, "/Users/private/project/slide.html") || agentErr.HTTPStatus() != 422 {
		t.Fatalf("trace/http projection mismatch: trace=%+v status=%d", trace, agentErr.HTTPStatus())
	}
}
