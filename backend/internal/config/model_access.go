package config

import (
	"errors"
	"net/url"
	"strings"
)

const (
	ProtocolResponses = "responses"
	ProtocolAnthropic = "anthropic"
)

func NormalizeModelBaseURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

// BaseURL is the complete API prefix. Adapters append only /responses or /messages.
func ValidateModelAccess(protocol, baseURL string) error {
	if protocol != ProtocolResponses && protocol != ProtocolAnthropic {
		return errors.New("MODEL_PROTOCOL_UNSUPPORTED: protocol must be responses or anthropic")
	}
	u, err := url.Parse(baseURL)
	if err != nil || u == nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(baseURL, " \t\r\n\\") {
		return errors.New("base_url must be an HTTP(S) API base address without credentials, query or fragment")
	}
	path := strings.TrimRight(u.Path, "/")
	if strings.HasSuffix(path, "/responses") || strings.HasSuffix(path, "/messages") || strings.HasSuffix(path, "/chat/completions") {
		return errors.New("base_url must be the API prefix, without /responses, /messages or /chat/completions")
	}
	return nil
}
