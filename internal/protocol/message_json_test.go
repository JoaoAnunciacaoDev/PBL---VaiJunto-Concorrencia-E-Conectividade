package protocol

import (
	"bufio"
	"errors"
	"strings"
	"testing"
)

func TestReadMessageEnforcesFiveMiBLimit(t *testing.T) {
	if MaxRequestSize != 5*1024*1024 {
		t.Fatalf("limite = %d; esperado 5 MiB", MaxRequestSize)
	}

	reader := bufio.NewReader(strings.NewReader(strings.Repeat("x", MaxRequestSize+1) + "\n"))
	var destination map[string]any
	if err := ReadMessage(reader, MaxRequestSize, &destination); !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("esperado ErrMessageTooLarge, recebido %v", err)
	}
}

func TestUnmarshalStrictRejectsUnknownAndTrailingFields(t *testing.T) {
	type envelope struct {
		Action string `json:"action"`
	}

	for _, input := range []string{
		`{"action":"login","extra":true}`,
		`{"action":"login"} {}`,
	} {
		var destination envelope
		if err := UnmarshalStrict([]byte(input), &destination); err == nil {
			t.Fatalf("mensagem deveria ser rejeitada: %s", input)
		}
	}
}
