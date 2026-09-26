package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

const CodeToolArgumentInvalid = "TOOL_ARGUMENT_INVALID"

func traceArgumentConversion(trace TraceRecorder, runID string, call llm.ToolCall, fields []string) {
	if len(fields) > 0 {
		recordTrace(trace, runID, "tool.arguments_normalized", map[string]any{
			"tool": call.Name, "call_id": call.ID, "fields": fields, "conversion": "json_string_to_array",
		})
	}
}

type toolArgumentError struct {
	Field, Expected, Actual, Message string
}

func (e *toolArgumentError) Error() string {
	return fmt.Sprintf("tool arguments do not match schema at %s: %s", e.Field, e.Message)
}

func compileToolArguments(schema ToolSchema, args map[string]any) (*jsonschema.Schema, error) {
	raw, err := json.Marshal(discriminatedArgumentSchema(schema.Parameters, args))
	if err != nil {
		return nil, fmt.Errorf("tool argument schema is invalid: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	const resource = "tool-arguments.schema.json"
	if err := compiler.AddResource(resource, io.NopCloser(bytes.NewReader(raw))); err != nil {
		return nil, fmt.Errorf("tool argument schema is invalid: %w", err)
	}
	compiled, err := compiler.Compile(resource)
	if err != nil {
		return nil, fmt.Errorf("tool argument schema is invalid: %w", err)
	}
	return compiled, nil
}

// Prepare a separate JSON value so rejected requests never partially alter the
// original arguments. Only schema-declared arrays accept encoded JSON arrays;
// strings such as Outline source, HTML and text anchors remain exact strings.
func prepareToolArguments(schema ToolSchema, args map[string]any) (map[string]any, []string, error) {
	if args == nil {
		args = map[string]any{}
	}
	compiled, err := compileToolArguments(schema, args)
	if err != nil {
		return nil, nil, err
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, nil, &toolArgumentError{Field: "/", Message: "arguments must be JSON values"}
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, nil, err
	}
	next, fields := normalizeArrayArguments(compiled, value, "", 0)
	if err := checkToolArguments(compiled, next); err != nil {
		return nil, nil, err
	}
	return next.(map[string]any), fields, nil
}

func normalizeArrayArguments(schema *jsonschema.Schema, value any, path string, depth int) (any, []string) {
	if schema == nil || depth > 64 || schema.Validate(value) == nil {
		return value, nil
	}
	fields := []string{}
	apply := func(child *jsonschema.Schema) {
		var changed []string
		value, changed = normalizeArrayArguments(child, value, path, depth+1)
		fields = append(fields, changed...)
	}
	if schema.Ref != nil {
		apply(schema.Ref)
	}
	for _, child := range schema.AllOf {
		apply(child)
	}
	for _, variants := range [][]*jsonschema.Schema{schema.OneOf, schema.AnyOf} {
		// Do not choose a conversion when multiple branches fit. A valid string
		// branch already returned above and is never coerced into an array.
		matches := 0
		var candidate any
		var changed []string
		for _, child := range variants {
			v, f := normalizeArrayArguments(child, value, path, depth+1)
			if child.Validate(v) == nil {
				matches++
				candidate, changed = v, f
			}
		}
		if matches == 1 {
			value = candidate
			fields = append(fields, changed...)
		}
	}
	if text, ok := value.(string); ok && len(schema.Types) == 1 && schema.Types[0] == "array" {
		var array []any
		// Reject duplicate keys, null, malformed JSON and double encoding.
		if spec.ValidateJSONSource([]byte(text)) == nil && json.Unmarshal([]byte(text), &array) == nil && array != nil {
			value = array
			fields = append(fields, argumentPath(path))
		}
	}
	switch current := value.(type) {
	case map[string]any:
		next := make(map[string]any, len(current))
		keys := make([]string, 0, len(current))
		for key := range current {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := schema.Properties[key]
			if child == nil {
				child, _ = schema.AdditionalProperties.(*jsonschema.Schema)
			}
			v, f := normalizeArrayArguments(child, current[key], path+"/"+strings.NewReplacer("~", "~0", "/", "~1").Replace(key), depth+1)
			next[key] = v
			fields = append(fields, f...)
		}
		value = next
	case []any:
		next := make([]any, len(current))
		for i, item := range current {
			child := schema.Items2020
			if i < len(schema.PrefixItems) {
				child = schema.PrefixItems[i]
			} else if child == nil {
				child, _ = schema.Items.(*jsonschema.Schema)
			}
			v, f := normalizeArrayArguments(child, item, fmt.Sprintf("%s/%d", path, i), depth+1)
			next[i] = v
			fields = append(fields, f...)
		}
		value = next
	}
	return value, fields
}

func argumentPath(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

func checkToolArguments(compiled *jsonschema.Schema, args any) error {
	if err := compiled.Validate(args); err != nil {
		var validation *jsonschema.ValidationError
		if errors.As(err, &validation) {
			leaf := mostSpecificValidationError(validation)
			out := &toolArgumentError{Field: argumentPath(leaf.InstanceLocation), Message: leaf.Message}
			// The validator supplies stable JSON type names without echoing data.
			if strings.HasSuffix(leaf.KeywordLocation, "/type") {
				out.Expected, out.Actual, _ = strings.Cut(strings.TrimPrefix(leaf.Message, "expected "), ", but got ")
				if out.Expected == "array" && out.Actual == "string" {
					out.Message = "expected a JSON array, got a string that could not be converted to an array; send a real array without outer quotes"
				}
			}
			return out
		}
		return &toolArgumentError{Field: "/", Message: err.Error()}
	}
	return nil
}

func argumentFailure(err error) ToolResult {
	var invalid *toolArgumentError
	if !errors.As(err, &invalid) {
		return failedToolResult("INTERNAL", err.Error(), false)
	}
	details := map[string]any{"field": invalid.Field}
	if invalid.Expected != "" {
		details["expected"] = invalid.Expected
		details["actual"] = invalid.Actual
	}
	return detailedToolFailure(CodeToolArgumentInvalid, invalid.Error(), details)
}
