package server

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"io"
	"net"
)

type Server struct {
	addr       string
	repository *Repository
}

func NewServer(addr string) *Server {
	return &Server{
		addr:       addr,
		repository: NewRepository(),
	}
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.addr)

	if err != nil {
		return fmt.Errorf("Erro ao iniciar o servidor: %v", err)
	}

	defer listener.Close()

	fmt.Printf("Servidor iniciado em %s\n", s.addr)

	for {
		conn, err := listener.Accept()

		if err != nil {
			fmt.Printf("Erro ao aceitar conexão: %v\n", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	for {
		var request protocol.Request
		err := protocol.ReadJson(reader, &request)

		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Println("Cliente encerrou a conexão.")
				return
			}

			fmt.Printf("Erro ao ler requisição: %v\n", err)
			return
		}

		res := s.processRequest(request)

		if err := protocol.SendJson(conn, res); err != nil {
			fmt.Printf("Erro ao enviar resposta: %v\n", err)
			return
		}
	}
}

func (s *Server) processRequest(request protocol.Request) protocol.Response {
	switch request.Action {
	case "register_user":
		return s.handleRegisterUser(request.Payload)

	case "login":
		return s.handleLogin(request.Payload)

	default:
		return protocol.Response{Success: "error", Message: "Ação desconhecida"}
	}
}
