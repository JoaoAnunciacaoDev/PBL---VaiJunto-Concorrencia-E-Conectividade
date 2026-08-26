package protocol

import "encoding/json"

type Response struct {
	Success string            `json:"success"`
	Message string          `json:"message,omitempty"`
	Payload    json.RawMessage `json:"data,omitempty"`
}