package protocol

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const MaxRequestSize = 5 * 1024 * 1024

var (
	ErrMessageTooLarge = errors.New("mensagem excede o limite máximo")
	ErrTrailingData    = errors.New("mensagem contém conteúdo adicional")
)

func SendJson(encoder *json.Encoder, data any) error {
	return encoder.Encode(data)
}

func ReadJson(decoder *json.Decoder, v any) error {
	return decoder.Decode(v)
}

// ReadMessage lê exatamente uma mensagem delimitada por quebra de linha. A
// conexão deve ser encerrada quando ErrMessageTooLarge for retornado, pois o
// restante da linha não precisa ser consumido.
func ReadMessage(reader *bufio.Reader, maxSize int, destination any) error {
	if maxSize <= 0 {
		return errors.New("limite de mensagem inválido")
	}

	message := make([]byte, 0, min(maxSize, 64*1024))
	for {
		fragment, isPrefix, err := reader.ReadLine()
		if len(message)+len(fragment) > maxSize {
			return fmt.Errorf("%w de %d bytes", ErrMessageTooLarge, maxSize)
		}
		message = append(message, fragment...)

		if err != nil {
			if errors.Is(err, io.EOF) && len(message) > 0 {
				break
			}
			return err
		}
		if !isPrefix {
			break
		}
	}

	return UnmarshalStrict(message, destination)
}

// UnmarshalStrict rejeita campos desconhecidos e qualquer segundo valor ou
// conteúdo não branco depois do objeto JSON esperado.
func UnmarshalStrict(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return ErrTrailingData
		}
		return fmt.Errorf("%w: %v", ErrTrailingData, err)
	}
	return nil
}
