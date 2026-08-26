package client

import (
	"bufio"
	"fmt"
	"net"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
)

type TCPClient struct {
	conn   net.Conn
	reader *bufio.Reader
}

func Dial(address string) (*TCPClient, error) {
	conn, err := net.Dial("tcp", address)

	if err != nil {
		return nil, fmt.Errorf("conectar ao servidor: %w", err)
	}

	return &TCPClient{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}, nil
}

func (c *TCPClient) Send(request protocol.Request) (protocol.Response, error) {
	if err := protocol.SendJson(c.conn, request); err != nil {
		return protocol.Response{}, fmt.Errorf("enviar requisição: %w", err)
	}

	var response protocol.Response
	
	if err := protocol.ReadJson(c.reader, &response); err != nil {
		return protocol.Response{}, fmt.Errorf("ler resposta: %w", err)
	}

	return response, nil
}

func (c *TCPClient) Close() error {
	return c.conn.Close()
}
