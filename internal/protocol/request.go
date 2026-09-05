package protocol

import "encoding/json"

type Request struct {
	Action  Action          `json:"action"`
	Payload json.RawMessage `json:"payload"`
}
