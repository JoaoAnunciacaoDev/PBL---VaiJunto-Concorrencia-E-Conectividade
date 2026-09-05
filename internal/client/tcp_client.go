package client

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
)

const (
	connectTimeout = 5 * time.Second
	requestTimeout = 10 * time.Second
)

type TCPClient struct {
	conn    net.Conn
	encoder *json.Encoder
	decoder *json.Decoder
}

func Dial(address string) (*TCPClient, error) {
	conn, err := net.DialTimeout("tcp", address, connectTimeout)

	if err != nil {
		return nil, fmt.Errorf("Erro ao conectar ao servidor: %w", err)
	}

	return &TCPClient{
		conn:    conn,
		encoder: json.NewEncoder(conn),
		decoder: json.NewDecoder(conn),
	}, nil
}

func (c *TCPClient) Send(request protocol.Request) (protocol.Response, error) {
	if err := c.conn.SetDeadline(time.Now().Add(requestTimeout)); err != nil {
		return protocol.Response{}, fmt.Errorf("definir tempo limite: %w", err)
	}
	
	defer c.conn.SetDeadline(time.Time{})

	if err := protocol.SendJson(c.encoder, request); err != nil {
		return protocol.Response{}, fmt.Errorf("enviar requisição: %w", err)
	}

	var response protocol.Response

	if err := protocol.ReadJson(c.decoder, &response); err != nil {
		return protocol.Response{}, fmt.Errorf("ler resposta: %w", err)
	}

	return response, nil
}

func (c *TCPClient) Close() error {
	return c.conn.Close()
}
