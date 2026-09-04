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

	case "get_my_vehicle":
		return s.handleGetMyVehicle(session)

	case "update_vehicle":
		return s.handleUpdateVehicle(request.Payload, session)

	case "remove_vehicle":
		return s.handleRemoveVehicle(session)

	case "create_ride":
		return s.handleCreateRide(request.Payload, session)

	case "list_my_rides":
		return s.handleListMyRides(session)

	case "cancel_ride":
		return s.handleCancelRide(request.Payload, session)

	case "search_itineraries":
		return s.handleSearchItineraries(request.Payload, session)

	case "confirm_reservation":
		return s.handleConfirmReservation(request.Payload, session)

	case "list_my_reservations":
		return s.handleListMyReservations(session)

	case "cancel_reservation":
		return s.handleCancelReservation(request.Payload, session)

	default:
		return protocol.Response{Success: "error", Message: "Ação desconhecida"}
	}
}
