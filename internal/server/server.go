package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
)

type Server struct {
	addr       string
	repository *Repository
}

// NewServer cria uma nova instância do servidor com o endereço e o caminho para o arquivo de usuários fornecidos.
// Ele inicializa o repositório de usuários
// Retorna um ponteiro para a instância do servidor e possíveis erros.
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

// Start inicia o servidor, escutando no endereço especificado e aceitando conexões de clientes.
// Para cada conexão aceita, ele cria uma goroutine para lidar com a comunicação com o cliente.
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.addr)

	if err != nil {
		return fmt.Errorf("Erro ao iniciar o servidor: %v", err)
	}

	defer listener.Close()

	log.Printf("server started address=%s", s.addr)

	for {
		conn, err := listener.Accept()

		if err != nil {
			log.Printf("accept connection failed error=%v", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

// handleConnection lida com a comunicação com um cliente conectado.
// Ele lê solicitações JSON do cliente, processa as solicitações e envia respostas JSON de volta.
// O loop continua até que o cliente se desconecte ou ocorra um erro.
func (s *Server) handleConnection(conn net.Conn) {
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)
	session := NewSession()
	remoteAddress := conn.RemoteAddr().String()

	log.Printf("connection opened remote=%s session=%s", remoteAddress, session.ID)

	defer func() {
		log.Printf("connection closed remote=%s session=%s user=%q", remoteAddress, session.ID, session.UserName)
		conn.Close()
	}()

	for {
		var request protocol.Request
		// Lê a solicitação JSON do cliente. apenas executa o restante do código
		// se houver o que ler e não houver erro na leitura da solicitação.
		err := protocol.ReadJson(decoder, &request)

		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("client disconnected remote=%s session=%s", remoteAddress, session.ID)
				return
			}

			log.Printf("request read failed remote=%s session=%s error=%v", remoteAddress, session.ID, err)
			return
		}

		userName := session.UserName
		res := s.processRequest(request, session)

		if userName == "" {
			userName = "anonymous"
			if session.UserName != "" {
				userName = session.UserName
			}
		}

		log.Printf("request action=%s session=%s user=%q result=%s", request.Action, session.ID, userName, res.Success)

		if err := protocol.SendJson(encoder, res); err != nil {
			log.Printf("response send failed remote=%s session=%s user=%q error=%v", remoteAddress, session.ID, userName, err)
			return
		}
	}
}

func (s *Server) processRequest(request protocol.Request, session *Session) protocol.Response {
	switch request.Action {
	case protocol.ActionRegisterUser:
		return s.handleRegisterUser(request.Payload)

	case protocol.ActionLogin:
		return s.handleLogin(request.Payload, session)

	case protocol.ActionGetMyProfile:
		return s.handleGetMyProfile(session)

	case protocol.ActionLogout:
		return s.handleLogout(session)

	case protocol.ActionRegisterVehicle:
		return s.handleRegisterVehicle(request.Payload, session)

	case protocol.ActionGetMyVehicle:
		return s.handleGetMyVehicle(session)

	case protocol.ActionUpdateVehicle:
		return s.handleUpdateVehicle(request.Payload, session)

	case protocol.ActionRemoveVehicle:
		return s.handleRemoveVehicle(session)

	case protocol.ActionCreateRide:
		return s.handleCreateRide(request.Payload, session)

	case protocol.ActionListMyRides:
		return s.handleListMyRides(session)

	case protocol.ActionCancelRide:
		return s.handleCancelRide(request.Payload, session)

	case protocol.ActionGetRidePassengers:
		return s.handleGetRidePassengers(request.Payload, session)

	case protocol.ActionSearchItineraries:
		return s.handleSearchItineraries(request.Payload, session)

	case protocol.ActionConfirmReservation:
		return s.handleConfirmReservation(request.Payload, session)

	case protocol.ActionListMyReservations:
		return s.handleListMyReservations(session)

	case protocol.ActionCancelReservation:
		return s.handleCancelReservation(request.Payload, session)

	default:
		return protocol.Response{Success: "error", Message: "Ação desconhecida"}
	}
}
