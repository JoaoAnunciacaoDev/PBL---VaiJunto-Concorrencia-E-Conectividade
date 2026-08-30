package protocol

import (
	"bufio"
	"encoding/json"
	"net"
)

func SendJson(conn net.Conn, data any) error {
	dataJSON, err := json.Marshal(data)

	if err != nil {
		return err
	}

	dataJSON = append(dataJSON, '\n')
	_, err = conn.Write(dataJSON)

	return err
}

func ReadJson(reader *bufio.Reader, v any) error {
	line, err := reader.ReadBytes('\n')

	if err != nil {
		return err
	}

	return json.Unmarshal(line, v)
}
