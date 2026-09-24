package config

import (
	"strings"
	"testing"
)

func TestModelAccessRequiresExplicitSupportedProtocolAndBaseURL(t *testing.T) {
	for _, protocol := range []string{ProtocolResponses, ProtocolAnthropic} {
		if err := ValidateModelAccess(protocol, "https://gateway.example/provider/v1"); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ protocol, url string }{
		{"", "https://gateway.example/v1"}, {"chat_completions", "https://gateway.example/v1"},
		{ProtocolResponses, ""}, {ProtocolResponses, "file:///tmp/model"},
		{ProtocolResponses, "https://user:secret@gateway.example/v1"},
		{ProtocolResponses, "https://gateway.example/v1?key=secret"},
		{ProtocolResponses, "https://gateway.example/v1/responses"},
		{ProtocolAnthropic, "https://gateway.example/v1/messages"},
	} {
		err := ValidateModelAccess(tc.protocol, tc.url)
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("invalid configuration accepted or exposed credentials: %v", err)
		}
	}
}
