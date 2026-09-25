package threadjournal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Prepare shares substantial text across input, display and model events. JSON
// pointers are envelope metadata, so a user payload cannot impersonate a reference.
// All referenced bytes are published and synced before the outbox transaction commits.
func Prepare(projectRoot, threadID string, event Event) (Event, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(event.Payload))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return event, err
	}
	fields := map[string]string{}
	var extract func(any, string) (any, error)
	extract = func(value any, pointer string) (any, error) {
		switch v := value.(type) {
		case string:
			if len(v) < 256 {
				return v, nil
			}
			raw, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			ref, err := Publish(projectRoot, threadID, raw)
			if err != nil {
				return nil, err
			}
			fields[pointer] = ref
			return nil, nil
		case map[string]any:
			for key, item := range v {
				next, err := extract(item, pointer+"/"+strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1"))
				if err != nil {
					return nil, err
				}
				v[key] = next
			}
		case []any:
			for i, item := range v {
				next, err := extract(item, pointer+"/"+strconv.Itoa(i))
				if err != nil {
					return nil, err
				}
				v[i] = next
			}
		}
		return value, nil
	}
	value, err := extract(value, "")
	if err != nil {
		return event, err
	}
	if len(fields) > 0 {
		event.Payload, err = json.Marshal(value)
		if err != nil {
			return event, err
		}
		event.PayloadFields = fields
	}
	if len(event.Payload) > InlineLimit {
		ref, err := Publish(projectRoot, threadID, event.Payload)
		if err != nil {
			return event, err
		}
		event.PayloadRef, event.Payload = ref, nil
	}
	return event, nil
}

func hydrateFields(path string, event *Event) error {
	if len(event.PayloadFields) == 0 {
		return nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(event.Payload))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	for pointer, ref := range event.PayloadFields {
		raw, err := readPayload(path, ref)
		if err != nil {
			return err
		}
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return fmt.Errorf("%w: text payload", ErrCorrupt)
		}
		if pointer == "" {
			if value != nil {
				return fmt.Errorf("%w: occupied payload pointer", ErrCorrupt)
			}
			value = text
			continue
		}
		if !strings.HasPrefix(pointer, "/") {
			return fmt.Errorf("%w: payload pointer", ErrCorrupt)
		}
		parts := strings.Split(pointer[1:], "/")
		current := value
		for i, part := range parts {
			key := strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
			last := i == len(parts)-1
			switch node := current.(type) {
			case map[string]any:
				next, exists := node[key]
				if !exists {
					return fmt.Errorf("%w: missing payload pointer", ErrCorrupt)
				}
				if last {
					if next != nil {
						return fmt.Errorf("%w: occupied payload pointer", ErrCorrupt)
					}
					node[key] = text
				} else {
					current = next
				}
			case []any:
				index, err := strconv.Atoi(key)
				if err != nil || index < 0 || index >= len(node) {
					return fmt.Errorf("%w: payload array pointer", ErrCorrupt)
				}
				if last {
					if node[index] != nil {
						return fmt.Errorf("%w: occupied payload pointer", ErrCorrupt)
					}
					node[index] = text
				} else {
					current = node[index]
				}
			default:
				return fmt.Errorf("%w: payload pointer target", ErrCorrupt)
			}
		}
	}
	raw, err := json.Marshal(value)
	event.Payload = raw
	return err
}
