package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/consortium"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/httpapi"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
)

const (
	defaultReadIdleTimeout = 5 * time.Minute
	defaultWriteTimeout    = 10 * time.Second
)

type ServerConfig struct {
	CompanyID   string
	TCPAddr     string
	HTTPAddr    string
	UsersPath   string
	Peers       []consortium.Peer
	LockTTL     time.Duration
	HTTPTimeout time.Duration
}

type Server struct {
	companyID   string
	addr        string
	httpAddr    string
	repository  *Repository
	peers       *consortium.PeerRegistry
	clock       *consortium.LamportClock
	wal         *consortium.WAL
	lockStore   *consortium.LockStore
	coordinator *consortium.Coordinator
	httpServer  *httpapi.HTTPServer
	httpClient  *http.Client

	listener      net.Listener
	listenerMu    sync.Mutex
	closed        atomic.Bool
	actualTCPAddr string

	readIdleTimeout time.Duration
	writeTimeout    time.Duration
	maxRequestSize  int
}

// NewServer cria uma nova instância do servidor com valores padrão para compatibilidade.
func NewServer(addr, usersPath, companyID string) (*Server, error) {
	return NewServerWithConfig(ServerConfig{
		CompanyID: companyID,
		TCPAddr:   addr,
		UsersPath: usersPath,
	})
}

// NewServerWithConfig inicializa o servidor completo com suporte a consórcio distribuído,
// relógio de Lamport, Write-Ahead Log e servidor REST inter-empresa.
func NewServerWithConfig(cfg ServerConfig) (*Server, error) {
	if cfg.CompanyID == "" {
		cfg.CompanyID = "empresa-a"
	}

	repository, err := NewRepository(cfg.UsersPath)
	if err != nil {
		return nil, fmt.Errorf("carregar repositório: %w", err)
	}

	dataDir := filepath.Dir(cfg.UsersPath)
	walPath := filepath.Join(dataDir, "wal.log")
	wal, err := consortium.NewWAL(walPath)
	if err != nil {
		return nil, fmt.Errorf("inicializar WAL: %w", err)
	}

	lockTTL := cfg.LockTTL
	if lockTTL <= 0 {
		lockTTL = consortium.DefaultLockTTL
	}

	httpTimeout := cfg.HTTPTimeout
	if httpTimeout <= 0 {
		httpTimeout = 3 * time.Second
	}

	clock := consortium.NewLamportClock(0)
	lockStore := consortium.NewLockStore(lockTTL)
	peers := consortium.NewPeerRegistry(cfg.Peers)

	coordinator := consortium.NewCoordinator(
		cfg.CompanyID,
		clock,
		peers,
		wal,
		lockStore,
		repository,
		httpTimeout,
	)

	var httpServer *httpapi.HTTPServer
	if cfg.HTTPAddr != "" {
		httpServer = httpapi.NewHTTPServer(
			cfg.CompanyID,
			cfg.HTTPAddr,
			repository,
			lockStore,
			wal,
			clock,
		)
	}

	return &Server{
		companyID:       cfg.CompanyID,
		addr:            cfg.TCPAddr,
		httpAddr:        cfg.HTTPAddr,
		repository:      repository,
		peers:           peers,
		clock:           clock,
		wal:             wal,
		lockStore:       lockStore,
		coordinator:     coordinator,
		httpServer:      httpServer,
		httpClient:      &http.Client{Timeout: httpTimeout},
		readIdleTimeout: defaultReadIdleTimeout,
		writeTimeout:    defaultWriteTimeout,
		maxRequestSize:  protocol.MaxRequestSize,
	}, nil
}

// Start inicia o servidor, escutando no endereço especificado e aceitando conexões de clientes.
// Para cada conexão aceita, ele cria uma goroutine para lidar com a comunicação com o cliente.
func (s *Server) Start() error {
	// 1. Recupera e retransmite commits pendentes de transações anteriores caso o servidor tenha caído
	if s.coordinator != nil {
		s.coordinator.RecoverPendingCommits()
	}

	// 2. Inicia o servidor HTTP Gin inter-empresa em background se configurado
	if s.httpServer != nil {
		go func() {
			if err := s.httpServer.Start(); err != nil {
				log.Printf("[server] encerramento do servidor HTTP: %v", err)
			}
		}()
	}

	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("Erro ao iniciar o servidor: %v", err)
	}

	s.listenerMu.Lock()
	s.listener = listener
	s.actualTCPAddr = listener.Addr().String()
	s.listenerMu.Unlock()

	defer func() {
		s.listenerMu.Lock()
		if s.listener != nil {
			_ = s.listener.Close()
			s.listener = nil
		}
		s.listenerMu.Unlock()
	}()

	log.Printf("server started address=%s company=%s http=%s", s.actualTCPAddr, s.companyID, s.httpAddr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if s.closed.Load() || errors.Is(err, net.ErrClosed) {
				return nil
			}
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
	reader := bufio.NewReader(conn)
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
		if err := conn.SetReadDeadline(time.Now().Add(s.readIdleTimeout)); err != nil {
			log.Printf("read deadline failed remote=%s session=%s error=%v", remoteAddress, session.ID, err)
			return
		}

		err := protocol.ReadMessage(reader, s.maxRequestSize, &request)

		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("client disconnected remote=%s session=%s", remoteAddress, session.ID)
				return
			}

			message := "Mensagem de protocolo inválida: " + err.Error()
			if netError, ok := err.(net.Error); ok && netError.Timeout() {
				message = "Conexão encerrada por tempo limite de leitura."
			}
			log.Printf("request read failed remote=%s session=%s error=%v", remoteAddress, session.ID, err)
			_ = s.sendResponse(conn, encoder, protocol.Response{Success: "error", Message: message})
			return
		}
		if request.Action == "" {
			_ = s.sendResponse(conn, encoder, protocol.Response{Success: "error", Message: "Mensagem de protocolo inválida: action é obrigatória."})
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

		if err := s.sendResponse(conn, encoder, res); err != nil {
			log.Printf("response send failed remote=%s session=%s user=%q error=%v", remoteAddress, session.ID, userName, err)
			return
		}
	}
}

// sendResponse envia uma resposta JSON para o cliente conectado.
func (s *Server) sendResponse(conn net.Conn, encoder *json.Encoder, response protocol.Response) error {
	if err := conn.SetWriteDeadline(time.Now().Add(s.writeTimeout)); err != nil {
		return err
	}
	defer conn.SetWriteDeadline(time.Time{})
	return protocol.SendJson(encoder, response)
}

// Close encerra os recursos persistentes abertos pelo servidor (como o WAL, listener TCP e HTTP server).
func (s *Server) Close() error {
	s.closed.Store(true)

	s.listenerMu.Lock()
	if s.listener != nil {
		_ = s.listener.Close()
		s.listener = nil
	}
	s.listenerMu.Unlock()

	if s.wal != nil {
		_ = s.wal.Close()
	}
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.httpServer.Stop(ctx)
	}
	return nil
}

// TCPAddr retorna o endereço TCP real onde o listener está escutando.
func (s *Server) TCPAddr() string {
	s.listenerMu.Lock()
	defer s.listenerMu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	if s.actualTCPAddr != "" {
		return s.actualTCPAddr
	}
	return s.addr
}

// HTTPAddr retorna o endereço HTTP real onde o servidor Gin está escutando.
func (s *Server) HTTPAddr() string {
	if s.httpServer != nil {
		return s.httpServer.Addr()
	}
	return s.httpAddr
}

// Repository expõe o repositório do servidor para inspeção em testes.
func (s *Server) Repository() *Repository {
	return s.repository
}

// LockStore expõe a store de locks do servidor para inspeção em testes.
func (s *Server) LockStore() *consortium.LockStore {
	return s.lockStore
}

// Clock expõe o relógio lógico de Lamport do servidor para inspeção em testes.
func (s *Server) Clock() *consortium.LamportClock {
	return s.clock
}

// Peers expõe o registro de peers para testes.
func (s *Server) Peers() *consortium.PeerRegistry {
	return s.peers
}

// Coordinator expõe o coordenador 2PC para testes.
func (s *Server) Coordinator() *consortium.Coordinator {
	return s.coordinator
}

// processRequest processa a solicitação recebida do cliente com base na ação especificada.
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
