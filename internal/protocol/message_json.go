package protocol

import (
	"encoding/json"
)

func SendJson(encoder *json.Encoder, data any) error {
	return encoder.Encode(data)
}

func ReadJson(decoder *json.Decoder, v any) error {
	return decoder.Decode(v)
}
