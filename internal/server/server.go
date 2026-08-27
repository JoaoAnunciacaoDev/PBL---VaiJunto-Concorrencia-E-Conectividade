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

func NewServer(addr, usersPath string) (*Server, error) {
	repository, err := NewRepository(usersPath)

	if err != nil {
		return nil, fmt.Errorf("carregar repositório: %w", err)
	}

	return &Server{
		addr:       addr,
		repository: repository,
	}, nil
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
	session := &Session{}

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

		res := s.processRequest(request, session)

		if err := protocol.SendJson(conn, res); err != nil {
			fmt.Printf("Erro ao enviar resposta: %v\n", err)
			return
		}
	}
}

func (s *Server) processRequest(request protocol.Request, session *Session) protocol.Response {
	switch request.Action {
	case "register_user":
		return s.handleRegisterUser(request.Payload)

	case "login":
		return s.handleLogin(request.Payload, session)

	case "get_my_profile":
		return s.handleGetMyProfile(session)

	case "logout":
		return s.handleLogout(session)

	case "register_vehicle":
		return s.handleRegisterVehicle(request.Payload, session)

	default:
		return protocol.Response{Success: "error", Message: "Ação desconhecida"}
	}
}
