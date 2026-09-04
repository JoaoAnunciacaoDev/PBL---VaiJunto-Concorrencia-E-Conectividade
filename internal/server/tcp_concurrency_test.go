package server

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// TestTCPConcurrentClientsReserveLastSeat exercises the same path used by the
// terminal clients: distinct TCP connections, JSON messages, login and then a
// simultaneous reservation attempt for one remaining seat.
func TestTCPConcurrentClientsReserveLastSeat(t *testing.T) {
	server := newTestServer(t)
	address := startTCPTestListener(t, server)

	password := "Senha@123"
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("gerar hash de teste: %v", err)
	}

	driver := &models.User{
		ID:           uuid.New(),
		Name:         "Motorista",
		Email:        "motorista@example.com",
		PasswordHash: string(passwordHash),
		Role:         models.RoleDriver,
	}
	if err := server.repository.SaveUser(driver); err != nil {
		t.Fatalf("salvar motorista: %v", err)
	}

	ride := reservationTestRide(1, 1)
	ride.DriverID = driver.ID
	if err := server.repository.SaveRide(ride); err != nil {
		t.Fatalf("salvar carona: %v", err)
	}

	const clientCount = 24
	type result struct {
		response protocol.Response
		err      error
	}
	results := make(chan result, clientCount)
	ready := make(chan struct{}, clientCount)
	startReservation := make(chan struct{})
	var clients sync.WaitGroup

	for index := 0; index < clientCount; index++ {
		passenger := &models.User{
			ID:           uuid.New(),
			Name:         fmt.Sprintf("Passageiro %d", index),
			Email:        fmt.Sprintf("passageiro-%d@example.com", index),
			PasswordHash: string(passwordHash),
			Role:         models.RolePassenger,
		}
		if err := server.repository.SaveUser(passenger); err != nil {
			t.Fatalf("salvar passageiro %d: %v", index, err)
		}

		clients.Add(1)
		go func(email string) {
			defer clients.Done()

			conn, err := net.DialTimeout("tcp", address, 2*time.Second)
			if err != nil {
				ready <- struct{}{}
				results <- result{err: fmt.Errorf("conectar: %w", err)}
				return
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				ready <- struct{}{}
				results <- result{err: fmt.Errorf("definir prazo: %w", err)}
				return
			}

			encoder := json.NewEncoder(conn)
			decoder := json.NewDecoder(conn)
			loginPayload, _ := json.Marshal(protocol.LoginRequest{Email: email, Password: password})
			loginResponse, err := sendTCPTestRequest(encoder, decoder, protocol.Request{Action: "login", Payload: loginPayload})
			if err != nil || loginResponse.Success != "success" {
				ready <- struct{}{}
				if err == nil {
					err = fmt.Errorf("login recusado: %s", loginResponse.Message)
				}
				results <- result{err: err}
				return
			}

			ready <- struct{}{}
			<-startReservation

			reservationPayload, _ := json.Marshal(protocol.ConfirmReservationRequest{
				Segments: []models.ReservedSegment{{RideID: ride.ID, SegmentID: ride.Segments[0].ID}},
			})
			response, err := sendTCPTestRequest(encoder, decoder, protocol.Request{Action: "confirm_reservation", Payload: reservationPayload})
			results <- result{response: response, err: err}
		}(passenger.Email)
	}

	for range clientCount {
		<-ready
	}
	close(startReservation)
	clients.Wait()
	close(results)

	successes := 0
	for result := range results {
		if result.err != nil {
			t.Errorf("cliente TCP falhou: %v", result.err)
			continue
		}
		if result.response.Success == "success" {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("reservas bem-sucedidas = %d; esperado exatamente 1", successes)
	}

	storedRide, err := server.repository.GetRideByID(ride.ID)
	if err != nil {
		t.Fatalf("consultar carona: %v", err)
	}
	if storedRide.Segments[0].AvailableSeats != 0 {
		t.Fatalf("assentos restantes = %d; esperado 0", storedRide.Segments[0].AvailableSeats)
	}
}

func startTCPTestListener(t *testing.T, server *Server) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("iniciar listener TCP: %v", err)
	}

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go server.handleConnection(conn)
		}
	}()

	t.Cleanup(func() {
		_ = listener.Close()
		<-finished
	})
	return listener.Addr().String()
}

func sendTCPTestRequest(encoder *json.Encoder, decoder *json.Decoder, request protocol.Request) (protocol.Response, error) {
	if err := protocol.SendJson(encoder, request); err != nil {
		return protocol.Response{}, err
	}

	var response protocol.Response
	if err := protocol.ReadJson(decoder, &response); err != nil {
		return protocol.Response{}, err
	}
	return response, nil
}
