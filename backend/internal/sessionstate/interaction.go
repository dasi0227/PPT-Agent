package sessionstate

import "encoding/json"

type InteractionView struct {
	ID              string          `json:"id"`
	ThreadID        string          `json:"thread_id"`
	RunID           string          `json:"run_id"`
	CallID          string          `json:"call_id"`
	Kind            string          `json:"kind"`
	State           string          `json:"state"`
	RequestRevision int64           `json:"request_revision,string"`
	EntityRevision  int64           `json:"entity_revision,string"`
	UpdatedSeq      int64           `json:"updated_seq,string"`
	Request         json.RawMessage `json:"request"`
	Answer          json.RawMessage `json:"answer"`
	DetailRef       string          `json:"detail_ref,omitempty"`
}
