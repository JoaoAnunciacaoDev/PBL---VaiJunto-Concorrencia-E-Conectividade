package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// TestTCPConcurrentClientsReserveLastSeat executa o mesmo caminho usado pelos clientes de terminal:
// conexões TCP distintas, mensagens JSON, login e, em seguida, uma tentativa simultânea de reserva para um
// assento restante.
func TestTCPConcurrentClientsReserveLastSeat(t *testing.T) {
	silenceServerLogs(t)
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
		duration time.Duration
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
			startedAt := time.Now()
			response, err := sendTCPTestRequest(encoder, decoder, protocol.Request{Action: "confirm_reservation", Payload: reservationPayload})
			results <- result{response: response, err: err, duration: measuredDuration(startedAt)}
		}(passenger.Email)
	}

	for range clientCount {
		<-ready
	}
	loadStartedAt := time.Now()
	close(startReservation)
	clients.Wait()
	loadDuration := measuredDuration(loadStartedAt)
	close(results)

	successes := 0
	rejections := 0
	durations := make([]time.Duration, 0, clientCount)
	for result := range results {
		if result.err != nil {
			t.Errorf("cliente TCP falhou: %v", result.err)
			continue
		}
		durations = append(durations, result.duration)
		if result.response.Success == "success" {
			successes++
		} else {
			rejections++
		}
	}
	if successes != 1 {
		t.Fatalf("reservas bem-sucedidas = %d; esperado exatamente 1", successes)
	}
	if rejections != clientCount-1 {
		t.Fatalf("reservas recusadas = %d; esperado %d", rejections, clientCount-1)
	}

	storedRide, err := server.repository.GetRideByID(ride.ID)
	if err != nil {
		t.Fatalf("consultar carona: %v", err)
	}
	if storedRide.Segments[0].AvailableSeats != 0 {
		t.Fatalf("assentos restantes = %d; esperado 0", storedRide.Segments[0].AvailableSeats)
	}
	if count := reservationCount(server.repository); count != 1 {
		t.Fatalf("reservas em memória = %d; esperado 1", count)
	}

	restartedRepository, err := NewRepository(filepath.Join(filepath.Dir(server.repository.statePath), "users.json"))
	if err != nil {
		t.Fatalf("reabrir estado persistido: %v", err)
	}
	if count := reservationCount(restartedRepository); count != 1 {
		t.Fatalf("reservas persistidas = %d; esperado 1", count)
	}

	metrics := calculateLatencyMetrics(durations, loadDuration)
	t.Logf(
		"métricas TCP: clientes=%d sucessos=%d recusas=%d mínimo=%s média=%s p50=%s p95=%s máximo=%s throughput=%.2f req/s",
		clientCount, successes, rejections, metrics.minimum, metrics.average,
		metrics.p50, metrics.p95, metrics.maximum, metrics.throughput,
	)
}

func TestTCPReservationIsAtomicAcrossMultipleSegments(t *testing.T) {
	silenceServerLogs(t)
	server := newTestServer(t)
	address := startTCPTestListener(t, server)

	const password = "Senha@123"
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("gerar hash: %v", err)
	}
	passenger := &models.User{
		ID: uuid.New(), Name: "Passageiro", Email: "atomic@example.com",
		PasswordHash: string(passwordHash), Role: models.RolePassenger,
	}
	if err := server.repository.SaveUser(passenger); err != nil {
		t.Fatalf("salvar passageiro: %v", err)
	}

	ride := reservationTestRide(1, 0)
	if err := server.repository.SaveRide(ride); err != nil {
		t.Fatalf("salvar carona: %v", err)
	}

	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		t.Fatalf("conectar: %v", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("definir prazo: %v", err)
	}
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	loginPayload, _ := json.Marshal(protocol.LoginRequest{Email: passenger.Email, Password: password})
	loginResponse, err := sendTCPTestRequest(encoder, decoder, protocol.Request{Action: protocol.ActionLogin, Payload: loginPayload})
	if err != nil || loginResponse.Success != "success" {
		t.Fatalf("login TCP falhou: resposta=%+v erro=%v", loginResponse, err)
	}

	reservationPayload, _ := json.Marshal(protocol.ConfirmReservationRequest{Segments: []models.ReservedSegment{
		{RideID: ride.ID, SegmentID: ride.Segments[0].ID},
		{RideID: ride.ID, SegmentID: ride.Segments[1].ID},
	}})
	response, err := sendTCPTestRequest(encoder, decoder, protocol.Request{
		Action: protocol.ActionConfirmReservation, Payload: reservationPayload,
	})
	if err != nil {
		t.Fatalf("confirmar reserva TCP: %v", err)
	}
	if response.Success != "error" {
		t.Fatalf("reserva multitrecho deveria falhar: %+v", response)
	}

	storedRide, err := server.repository.GetRideByID(ride.ID)
	if err != nil {
		t.Fatalf("consultar carona: %v", err)
	}
	if storedRide.Segments[0].AvailableSeats != 1 || storedRide.Segments[1].AvailableSeats != 0 {
		t.Fatalf("assentos alterados parcialmente: %+v", storedRide.Segments)
	}
	if count := reservationCount(server.repository); count != 0 {
		t.Fatalf("reservas em memória = %d; esperado 0", count)
	}

	restartedRepository, err := NewRepository(filepath.Join(filepath.Dir(server.repository.statePath), "users.json"))
	if err != nil {
		t.Fatalf("reabrir estado: %v", err)
	}
	persistedRide, err := restartedRepository.GetRideByID(ride.ID)
	if err != nil {
		t.Fatalf("consultar carona persistida: %v", err)
	}
	if persistedRide.Segments[0].AvailableSeats != 1 || persistedRide.Segments[1].AvailableSeats != 0 {
		t.Fatalf("estado persistido parcialmente: %+v", persistedRide.Segments)
	}
	if count := reservationCount(restartedRepository); count != 0 {
		t.Fatalf("reservas persistidas = %d; esperado 0", count)
	}
}

type latencyMetrics struct {
	minimum    time.Duration
	average    time.Duration
	p50        time.Duration
	p95        time.Duration
	maximum    time.Duration
	throughput float64
}

func calculateLatencyMetrics(durations []time.Duration, wallTime time.Duration) latencyMetrics {
	if len(durations) == 0 || wallTime <= 0 {
		return latencyMetrics{}
	}

	sorted := append([]time.Duration(nil), durations...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	var total time.Duration
	for _, duration := range sorted {
		total += duration
	}

	percentile := func(percent float64) time.Duration {
		index := int(math.Ceil(percent*float64(len(sorted)))) - 1
		index = max(0, min(index, len(sorted)-1))
		return sorted[index]
	}

	return latencyMetrics{
		minimum: sorted[0], average: total / time.Duration(len(sorted)),
		p50: percentile(0.50), p95: percentile(0.95), maximum: sorted[len(sorted)-1],
		throughput: float64(len(sorted)) / wallTime.Seconds(),
	}
}

// measuredDuration espera a próxima leitura mensurável em ambientes cujo
// relógio tem resolução inferior à duração da operação. Assim, a métrica fica
// limitada pela resolução do relógio, mas nunca produz latência ou throughput
// zerados.
func measuredDuration(startedAt time.Time) time.Duration {
	for {
		if duration := time.Since(startedAt); duration > 0 {
			return duration
		}
		runtime.Gosched()
	}
}

func reservationCount(repository *Repository) int {
	repository.mu.RLock()
	defer repository.mu.RUnlock()
	return len(repository.reservations)
}

func silenceServerLogs(t *testing.T) {
	t.Helper()
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(io.Discard)
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	})
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
