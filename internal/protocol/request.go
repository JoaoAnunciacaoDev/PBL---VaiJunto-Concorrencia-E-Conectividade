package protocol

import "encoding/json"

type Request struct {
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload"`
}
