package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/consortium"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/google/uuid"
)

type mockRepo struct {
	rides     map[uuid.UUID]*models.Ride
	appliedTx map[string]bool
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		rides:     make(map[uuid.UUID]*models.Ride),
		appliedTx: make(map[string]bool),
	}
}

func (m *mockRepo) GetActiveRidesByDate(date time.Time) ([]*models.Ride, error) {
	var list []*models.Ride
	for _, r := range m.rides {
		list = append(list, r)
	}
	return list, nil
}

func (m *mockRepo) GetRideByID(id uuid.UUID) (*models.Ride, error) {
	r, exists := m.rides[id]
	if !exists {
		return nil, errors.New("not found")
	}
	return r, nil
}

func (m *mockRepo) GetRideBySegmentID(segmentID uuid.UUID) (*models.Ride, error) {
	for _, r := range m.rides {
		for _, s := range r.Segments {
			if s.ID == segmentID {
				return r, nil
			}
		}
	}
	return nil, errors.New("ride by segment not found")
}

func (m *mockRepo) DecrementSeatIdempotent(rideID, segmentID uuid.UUID, txID string) error {
	key := ""
	if txID != "" {
		key = txID + ":" + segmentID.String()
		if m.appliedTx[key] {
			return nil
		}
	}

	r, exists := m.rides[rideID]
	if !exists {
		return errors.New("ride not found")
	}
	for i := range r.Segments {
		if r.Segments[i].ID == segmentID {
			if r.Segments[i].AvailableSeats <= 0 {
				return errors.New("no seats")
			}
			r.Segments[i].AvailableSeats--
			if key != "" {
				m.appliedTx[key] = true
			}
			return nil
		}
	}
	return errors.New("segment not found")
}

func (m *mockRepo) DecrementSeat(rideID, segmentID uuid.UUID) error {
	return m.DecrementSeatIdempotent(rideID, segmentID, "")
}

func (m *mockRepo) IsTxApplied(txID string, segmentID uuid.UUID) bool {
	if txID == "" {
		return false
	}
	return m.appliedTx[txID+":"+segmentID.String()]
}

func (m *mockRepo) IncrementSeat(rideID, segmentID uuid.UUID) error {
	r, exists := m.rides[rideID]
	if !exists {
		return errors.New("ride not found")
	}
	for i := range r.Segments {
		if r.Segments[i].ID == segmentID {
			r.Segments[i].AvailableSeats++
			return nil
		}
	}
	return errors.New("segment not found")
}

func setupTestServer(t *testing.T) (*HTTPServer, *mockRepo, *consortium.WAL, *consortium.LockStore) {
	t.Helper()
	repo := newMockRepo()
	lockStore := consortium.NewLockStore(5 * time.Second)
	walPath := filepath.Join(t.TempDir(), "wal.log")
	wal, err := consortium.NewWAL(walPath)
	if err != nil {
		t.Fatalf("erro ao criar WAL: %v", err)
	}
	t.Cleanup(func() { wal.Close() })
	clock := consortium.NewLamportClock(0)

	server := NewHTTPServer("empresa-a", ":8081", repo, lockStore, wal, clock)
	return server, repo, wal, lockStore
}

func TestHealthEndpoint(t *testing.T) {
	server, _, _, _ := setupTestServer(t)

	req, _ := http.NewRequest(http.MethodGet, "/internal/v1/health", nil)
	resp := httptest.NewRecorder()
	server.Engine().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("esperado status 200, obtido %d", resp.Code)
	}

	var data healthResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatalf("falha ao decodificar resposta: %v", err)
	}

	if data.CompanyID != "empresa-a" || data.Status != "ok" {
		t.Fatalf("conteúdo incorreto: %+v", data)
	}
}

func TestGetRidesEndpoint(t *testing.T) {
	server, repo, _, _ := setupTestServer(t)
	now := time.Now()
	rideID := uuid.New()
	repo.rides[rideID] = &models.Ride{
		ID:          rideID,
		DriverID:    uuid.New(),
		DepartureAt: now,
		Segments: []models.Stage{
			{ID: uuid.New(), Origin: enum.Salvador, Destination: enum.FeiraDeSantana, AvailableSeats: 2, DepartureAt: now, ArrivalAt: now.Add(time.Hour)},
		},
	}

	req, _ := http.NewRequest(http.MethodGet, "/internal/v1/rides?date=2026-10-19", nil)
	resp := httptest.NewRecorder()
	server.Engine().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("esperado status 200, obtido %d", resp.Code)
	}

	var rides []models.Ride
	if err := json.Unmarshal(resp.Body.Bytes(), &rides); err != nil {
		t.Fatalf("falha ao decodificar caronas: %v", err)
	}
	if len(rides) != 1 {
		t.Fatalf("esperado 1 carona, obtido %d", len(rides))
	}
}

func Test2PCPrepareCommitAbortFlow(t *testing.T) {
	server, repo, _, _ := setupTestServer(t)
	now := time.Now()
	rideID := uuid.New()
	segmentID := uuid.New()
	repo.rides[rideID] = &models.Ride{
		ID:          rideID,
		DriverID:    uuid.New(),
		DepartureAt: now,
		Segments: []models.Stage{
			{ID: segmentID, Origin: enum.Salvador, Destination: enum.FeiraDeSantana, AvailableSeats: 1, DepartureAt: now, ArrivalAt: now.Add(time.Hour)},
		},
	}

	// 1. Prepare com sucesso
	prepBody, _ := json.Marshal(PrepareRequest{
		TxID:                 "tx-123",
		CoordinatorCompanyID: "empresa-b",
		LamportClock:         5,
		PassengerID:          uuid.New(),
		RideID:               rideID,
	})
	req, _ := http.NewRequest(http.MethodPost, "/internal/v1/segments/"+segmentID.String()+"/prepare", bytes.NewReader(prepBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.Engine().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("esperado prepare 200, obtido %d: %s", resp.Code, resp.Body.String())
	}

	// 2. Conflito: tentativa concorrente com Lamport maior (10) para o mesmo assento
	prepConflict, _ := json.Marshal(PrepareRequest{
		TxID:                 "tx-456",
		CoordinatorCompanyID: "empresa-c",
		LamportClock:         10,
		PassengerID:          uuid.New(),
		RideID:               rideID,
	})
	req2, _ := http.NewRequest(http.MethodPost, "/internal/v1/segments/"+segmentID.String()+"/prepare", bytes.NewReader(prepConflict))
	req2.Header.Set("Content-Type", "application/json")
	resp2 := httptest.NewRecorder()
	server.Engine().ServeHTTP(resp2, req2)

	if resp2.Code != http.StatusConflict {
		t.Fatalf("esperado 409 no conflito, obtido %d", resp2.Code)
	}

	// 3. Commit de tx-123
	commitBody, _ := json.Marshal(CommitRequest{
		TxID:                 "tx-123",
		CoordinatorCompanyID: "empresa-b",
		RideID:               rideID,
	})
	req3, _ := http.NewRequest(http.MethodPost, "/internal/v1/segments/"+segmentID.String()+"/commit", bytes.NewReader(commitBody))
	req3.Header.Set("Content-Type", "application/json")
	resp3 := httptest.NewRecorder()
	server.Engine().ServeHTTP(resp3, req3)

	if resp3.Code != http.StatusOK {
		t.Fatalf("esperado commit 200, obtido %d: %s", resp3.Code, resp3.Body.String())
	}

	// Verifica se o assento foi decrementado
	if repo.rides[rideID].Segments[0].AvailableSeats != 0 {
		t.Fatalf("assento deveria ter sido decrementado para 0, obtido %d", repo.rides[rideID].Segments[0].AvailableSeats)
	}

	// 4. Tentativa de novo prepare para mesmo segmento agora com 0 vagas -> deve dar 409
	req4, _ := http.NewRequest(http.MethodPost, "/internal/v1/segments/"+segmentID.String()+"/prepare", bytes.NewReader(prepConflict))
	req4.Header.Set("Content-Type", "application/json")
	resp4 := httptest.NewRecorder()
	server.Engine().ServeHTTP(resp4, req4)

	if resp4.Code != http.StatusConflict {
		t.Fatalf("esperado 409 sem vagas, obtido %d", resp4.Code)
	}
}

func Test2PCAbortFlow(t *testing.T) {
	server, repo, _, _ := setupTestServer(t)
	now := time.Now()
	rideID := uuid.New()
	segmentID := uuid.New()
	repo.rides[rideID] = &models.Ride{
		ID:          rideID,
		DriverID:    uuid.New(),
		DepartureAt: now,
		Segments: []models.Stage{
			{ID: segmentID, Origin: enum.Salvador, Destination: enum.FeiraDeSantana, AvailableSeats: 1, DepartureAt: now, ArrivalAt: now.Add(time.Hour)},
		},
	}

	// Prepare
	prepBody, _ := json.Marshal(PrepareRequest{
		TxID:                 "tx-abort-test",
		CoordinatorCompanyID: "empresa-b",
		LamportClock:         5,
		PassengerID:          uuid.New(),
		RideID:               rideID,
	})
	req, _ := http.NewRequest(http.MethodPost, "/internal/v1/segments/"+segmentID.String()+"/prepare", bytes.NewReader(prepBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.Engine().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("esperado prepare 200, obtido %d", resp.Code)
	}

	// Abort
	abortBody, _ := json.Marshal(AbortRequest{
		TxID:                 "tx-abort-test",
		CoordinatorCompanyID: "empresa-b",
	})
	reqAbort, _ := http.NewRequest(http.MethodPost, "/internal/v1/segments/"+segmentID.String()+"/abort", bytes.NewReader(abortBody))
	reqAbort.Header.Set("Content-Type", "application/json")
	respAbort := httptest.NewRecorder()
	server.Engine().ServeHTTP(respAbort, reqAbort)
	if respAbort.Code != http.StatusOK {
		t.Fatalf("esperado abort 200, obtido %d", respAbort.Code)
	}

	// Assento deve continuar intacto (1 vaga)
	if repo.rides[rideID].Segments[0].AvailableSeats != 1 {
		t.Fatalf("assento deveria continuar 1 após abort, obtido %d", repo.rides[rideID].Segments[0].AvailableSeats)
	}

	// Lock deve ter sido liberado, permitindo novo prepare
	respRePrep := httptest.NewRecorder()
	reqRePrep, _ := http.NewRequest(http.MethodPost, "/internal/v1/segments/"+segmentID.String()+"/prepare", bytes.NewReader(prepBody))
	reqRePrep.Header.Set("Content-Type", "application/json")
	server.Engine().ServeHTTP(respRePrep, reqRePrep)
	if respRePrep.Code != http.StatusOK {
		t.Fatalf("esperado novo prepare com sucesso após abort, obtido %d", respRePrep.Code)
	}
}

// TestPrepareWALWriteFailureRejectsVoteAndReleasesLock testa que se a escrita no WAL
// falhar durante o handlePrepare, o servidor responde HTTP 500 e libera o lock imediatamente.
func TestPrepareWALWriteFailureRejectsVoteAndReleasesLock(t *testing.T) {
	server, repo, wal, lockStore := setupTestServer(t)
	now := time.Now()
	rideID := uuid.New()
	segmentID := uuid.New()
	repo.rides[rideID] = &models.Ride{
		ID:          rideID,
		DriverID:    uuid.New(),
		DepartureAt: now,
		Segments: []models.Stage{
			{ID: segmentID, Origin: enum.Salvador, Destination: enum.FeiraDeSantana, AvailableSeats: 1, DepartureAt: now, ArrivalAt: now.Add(time.Hour)},
		},
	}

	// Fecha intencionalmente o arquivo do WAL para forçar erro de escrita I/O
	_ = wal.Close()

	prepBody, _ := json.Marshal(PrepareRequest{
		TxID:                 "tx-wal-fail",
		CoordinatorCompanyID: "empresa-b",
		LamportClock:         5,
		PassengerID:          uuid.New(),
		RideID:               rideID,
	})
	req, _ := http.NewRequest(http.MethodPost, "/internal/v1/segments/"+segmentID.String()+"/prepare", bytes.NewReader(prepBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.Engine().ServeHTTP(resp, req)

	// O voto DEVE ser rejeitado com HTTP 500
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("esperado status 500 por falha de WAL, obtido %d", resp.Code)
	}

	// O lock DEVE ter sido liberado
	if _, exists := lockStore.Get(segmentID.String()); exists {
		t.Fatal("lock não foi liberado após falha de escrita no WAL")
	}
}

// TestLateAbortDoesNotRevertCommitted valida que um abort tardio para uma transação
// que já é COMMITTED responde 200 OK (no-op), não altera o WAL para ABORTED e não toca no assento.
func TestLateAbortDoesNotRevertCommitted(t *testing.T) {
	server, repo, wal, _ := setupTestServer(t)
	now := time.Now()
	rideID := uuid.New()
	segmentID := uuid.New()
	repo.rides[rideID] = &models.Ride{
		ID:          rideID,
		DriverID:    uuid.New(),
		DepartureAt: now,
		Segments: []models.Stage{
			{ID: segmentID, Origin: enum.Salvador, Destination: enum.FeiraDeSantana, AvailableSeats: 2, DepartureAt: now, ArrivalAt: now.Add(time.Hour)},
		},
	}

	txID := "tx-late-abort"

	// 1. Prepare
	prepBody, _ := json.Marshal(PrepareRequest{
		TxID:                 txID,
		CoordinatorCompanyID: "empresa-b",
		LamportClock:         1,
		PassengerID:          uuid.New(),
		RideID:               rideID,
	})
	reqPrep, _ := http.NewRequest(http.MethodPost, "/internal/v1/segments/"+segmentID.String()+"/prepare", bytes.NewReader(prepBody))
	respPrep := httptest.NewRecorder()
	server.Engine().ServeHTTP(respPrep, reqPrep)
	if respPrep.Code != http.StatusOK {
		t.Fatalf("prepare falhou: %d", respPrep.Code)
	}

	// 2. Commit
	commitBody, _ := json.Marshal(CommitRequest{
		TxID:                 txID,
		CoordinatorCompanyID: "empresa-b",
		RideID:               rideID,
	})
	reqCommit, _ := http.NewRequest(http.MethodPost, "/internal/v1/segments/"+segmentID.String()+"/commit", bytes.NewReader(commitBody))
	respCommit := httptest.NewRecorder()
	server.Engine().ServeHTTP(respCommit, reqCommit)
	if respCommit.Code != http.StatusOK {
		t.Fatalf("commit falhou: %d", respCommit.Code)
	}

	// Vagas foram para 1
	if repo.rides[rideID].Segments[0].AvailableSeats != 1 {
		t.Fatalf("assento deveria ser 1, obtido %d", repo.rides[rideID].Segments[0].AvailableSeats)
	}

	// 3. Abort atrasado
	abortBody, _ := json.Marshal(AbortRequest{
		TxID:                 txID,
		CoordinatorCompanyID: "empresa-b",
	})
	reqAbort, _ := http.NewRequest(http.MethodPost, "/internal/v1/segments/"+segmentID.String()+"/abort", bytes.NewReader(abortBody))
	respAbort := httptest.NewRecorder()
	server.Engine().ServeHTTP(respAbort, reqAbort)
	if respAbort.Code != http.StatusOK {
		t.Fatalf("abort tardio deveria responder 200, obtido %d", respAbort.Code)
	}

	// Vagas devem continuar em 1 (não voltam para 2!)
	if repo.rides[rideID].Segments[0].AvailableSeats != 1 {
		t.Fatalf("FALHA: abort tardio incrementou vagas indevidamente! obtido %d", repo.rides[rideID].Segments[0].AvailableSeats)
	}

	// Estado consolidado no WAL deve continuar COMMITTED
	state, _, err := wal.GetTxState(txID)
	if err != nil || state != consortium.TxCommitted {
		t.Fatalf("estado no WAL foi corrompido: state=%s err=%v", state, err)
	}
}

// TestParticipantRecoveryAppliesPendingCommit valida que, se o participante registrou COMMITTED
// no WAL mas caiu antes do efeito no repositório, o RecoverParticipant reaplica o efeito no reinício.
func TestParticipantRecoveryAppliesPendingCommit(t *testing.T) {
	server, repo, wal, _ := setupTestServer(t)
	now := time.Now()
	rideID := uuid.New()
	segmentID := uuid.New()
	repo.rides[rideID] = &models.Ride{
		ID:          rideID,
		DriverID:    uuid.New(),
		DepartureAt: now,
		Segments: []models.Stage{
			{ID: segmentID, Origin: enum.Salvador, Destination: enum.FeiraDeSantana, AvailableSeats: 2, DepartureAt: now, ArrivalAt: now.Add(time.Hour)},
		},
	}

	txID := "tx-pending-effect"

	// Simula entrada no WAL COMMITTED para o participante (mas sem chamar decrementSeat ainda)
	_ = wal.Append(consortium.WALEntry{
		TxID:          txID,
		Role:          "participant",
		Status:        consortium.TxCommitted,
		CoordinatorID: "empresa-coord",
		SegmentIDs:    []string{segmentID.String()},
		UpdatedAt:     time.Now(),
	})

	// Assentos ainda estão em 2
	if repo.rides[rideID].Segments[0].AvailableSeats != 2 {
		t.Fatalf("assento deveria ser 2 antes do recovery")
	}

	// Executa a recuperação do participante
	server.RecoverParticipant()

	// Assentos agora devem ter sido debitados para 1!
	if repo.rides[rideID].Segments[0].AvailableSeats != 1 {
		t.Fatalf("assento deveria ser debitado pelo recovery para 1, obtido %d", repo.rides[rideID].Segments[0].AvailableSeats)
	}

	// E se rodar o recovery novamente, a idempotência garante que continua em 1
	server.RecoverParticipant()
	if repo.rides[rideID].Segments[0].AvailableSeats != 1 {
		t.Fatalf("assento não deveria ser debitado duas vezes, obtido %d", repo.rides[rideID].Segments[0].AvailableSeats)
	}
}
