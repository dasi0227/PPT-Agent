package llm

import (
	"errors"
	"fmt"
)

// ToolStrategy reports the actual adapter strategy, including routed fallback.
// EndpointKey is hashed so diagnostics never disclose a private gateway URL.
type ToolStrategy struct {
	Provider          string `json:"provider"`
	Protocol          string `json:"protocol"`
	Model             string `json:"model"`
	EndpointKey       string `json:"endpoint_key"`
	RequiredTool      string `json:"required_tool,omitempty"`
	ParallelToolCalls *bool  `json:"parallel_tool_calls,omitempty"`
}
type constraintIdentity struct{ provider, protocol, model, endpoint string }

func (s ToolStrategy) identity() constraintIdentity {
	return constraintIdentity{s.Provider, s.Protocol, s.Model, s.EndpointKey}
}

// One policy belongs to one sequential operation; it is never persisted.
type ToolConstraintPolicy struct {
	unsupported map[constraintIdentity]map[string]bool
}

type UnsupportedToolConstraintError struct {
	Strategy ToolStrategy
	Field    string
	Cause    error
}

func (e *UnsupportedToolConstraintError) Error() string {
	return fmt.Sprintf("unsupported tool constraint %s: %v", e.Field, e.Cause)
}
func (e *UnsupportedToolConstraintError) Unwrap() error { return e.Cause }

func (p *ToolConstraintPolicy) Learn(err error) bool {
	var unsupported *UnsupportedToolConstraintError
	if p == nil || !errors.As(err, &unsupported) {
		return false
	}
	if p.unsupported == nil {
		p.unsupported = map[constraintIdentity]map[string]bool{}
	}
	key := unsupported.Strategy.identity()
	if p.unsupported[key] == nil {
		p.unsupported[key] = map[string]bool{}
	}
	if p.unsupported[key][unsupported.Field] {
		return false
	}
	p.unsupported[key][unsupported.Field] = true
	return true
}

func toolStrategy(req GenerateRequest, provider, protocol, model, endpoint string) (ToolStrategy, error) {
	s := ToolStrategy{Provider: provider, Protocol: protocol, Model: model, EndpointKey: continuationFingerprint(endpoint), RequiredTool: req.RequiredTool, ParallelToolCalls: req.ParallelToolCalls}
	if req.RequiredTool != "" {
		found := false
		for _, tool := range req.Tools {
			if tool.Name == req.RequiredTool {
				found = true
			}
		}
		if !found {
			return s, fmt.Errorf("%w: required tool was not disclosed", ErrBadRequest)
		}
	}
	if req.ToolConstraintPolicy != nil {
		known := req.ToolConstraintPolicy.unsupported[s.identity()]
		if known["tool_choice"] || known["tool_choice_object"] {
			s.RequiredTool = ""
		}
		if known["parallel_tool_calls"] {
			s.ParallelToolCalls = nil
		}
		// Anthropic's parallel limit cannot be sent without a tool_choice object.
		if protocol == ProtocolAnthropic && known["tool_choice_object"] {
			s.ParallelToolCalls = nil
		}
	}
	if req.OnToolStrategy != nil {
		req.OnToolStrategy(s)
	}
	return s, nil
}

func classifyToolConstraintError(err error, s ToolStrategy) error {
	var upstream *ProviderError
	if !errors.As(err, &upstream) || upstream.StatusCode != 400 {
		return err
	}
	// Both unsupported semantics and the exact field need machine evidence.
	// A generic 400, invalid_request_error or free-text message is insufficient.
	switch upstream.Code {
	case "unsupported_parameter", "unsupported_value", "not_supported":
	default:
		return err
	}
	field := ""
	switch upstream.Param {
	case "tool_choice", "tool_choice.type":
		if s.RequiredTool != "" || (s.Protocol == ProtocolAnthropic && s.ParallelToolCalls != nil) {
			field = "tool_choice"
		}
		if field != "" && upstream.Param == "tool_choice" && upstream.Code == "unsupported_parameter" {
			field = "tool_choice_object"
		}
	case "parallel_tool_calls":
		if s.Protocol == ProtocolResponses && s.ParallelToolCalls != nil {
			field = "parallel_tool_calls"
		}
	case "tool_choice.disable_parallel_tool_use":
		if s.Protocol == ProtocolAnthropic && s.ParallelToolCalls != nil {
			field = "parallel_tool_calls"
		}
	}
	if field == "" {
		return err
	}
	return &UnsupportedToolConstraintError{Strategy: s, Field: field, Cause: err}
}
