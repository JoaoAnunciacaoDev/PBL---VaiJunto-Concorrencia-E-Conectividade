package consortium

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/google/uuid"
)

type coordinatorMockRepo struct {
	rides map[uuid.UUID]*models.Ride
}

func (m *coordinatorMockRepo) GetRideByID(id uuid.UUID) (*models.Ride, error) {
	r, exists := m.rides[id]
	if !exists {
		return nil, errors.New("not found")
	}
	return r, nil
}

func (m *coordinatorMockRepo) DecrementSeat(rideID, segmentID uuid.UUID) error {
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
			return nil
		}
	}
	return errors.New("segment not found")
}

func (m *coordinatorMockRepo) IncrementSeat(rideID, segmentID uuid.UUID) error {
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

func setupCoordinator(t *testing.T, companyID string, peers []Peer) (*Coordinator, *coordinatorMockRepo, *LockStore, *WAL) {
	t.Helper()
	clock := NewLamportClock(0)
	peerReg := NewPeerRegistry(peers)
	walPath := filepath.Join(t.TempDir(), "wal.log")
	wal, err := NewWAL(walPath)
	if err != nil {
		t.Fatalf("erro ao criar WAL: %v", err)
	}
	t.Cleanup(func() { wal.Close() })
	lockStore := NewLockStore(5 * time.Second)
	repo := &coordinatorMockRepo{rides: make(map[uuid.UUID]*models.Ride)}

	coord := NewCoordinator(companyID, clock, peerReg, wal, lockStore, repo, 1*time.Second)
	return coord, repo, lockStore, wal
}

func TestCoordinatorConfirmLocalItinerary(t *testing.T) {
	coord, repo, lockStore, _ := setupCoordinator(t, "empresa-a", nil)

	rideID := uuid.New()
	segID := uuid.New()
	repo.rides[rideID] = &models.Ride{
		ID:       rideID,
		DriverID: uuid.New(),
		Segments: []models.Stage{
			{ID: segID, Origin: enum.Salvador, Destination: enum.FeiraDeSantana, AvailableSeats: 1},
		},
	}

	segments := []models.ReservedSegment{
		{RideID: rideID, SegmentID: segID, CompanyID: "empresa-a"},
	}

	passengerID := uuid.New()
	if err := coord.ConfirmItinerary(passengerID, segments); err != nil {
		t.Fatalf("confirmação local deveria suceder: %v", err)
	}

	// Assento deve ter sido decrementado
	if repo.rides[rideID].Segments[0].AvailableSeats != 0 {
		t.Fatalf("assento deveria ser 0, obtido %d", repo.rides[rideID].Segments[0].AvailableSeats)
	}

	// Lock local deve ter sido liberado após o commit
	if _, exists := lockStore.Get(segID.String()); exists {
		t.Fatal("lock local deveria ser liberado após o commit")
	}
}

func TestCoordinatorDistributedItinerarySuccess(t *testing.T) {
	// Simula servidor da empresa parceira (empresa-b)
	prepareCalled := false
	commitCalled := false

	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/v1/segments/seg-b/prepare":
			prepareCalled = true
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "lamport_clock": 5})
		case "/internal/v1/segments/seg-b/commit":
			commitCalled = true
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			http.NotFound(w, r)
		}
	}))
	defer remoteServer.Close()

	peers := []Peer{{CompanyID: "empresa-b", InternalURL: remoteServer.URL}}
	coord, repo, _, _ := setupCoordinator(t, "empresa-a", peers)

	localRideID := uuid.New()
	localSegID := uuid.New()
	repo.rides[localRideID] = &models.Ride{
		ID: localRideID,
		Segments: []models.Stage{
			{ID: localSegID, AvailableSeats: 1},
		},
	}

	segments := []models.ReservedSegment{
		{RideID: localRideID, SegmentID: localSegID, CompanyID: "empresa-a"},
		{RideID: uuid.New(), SegmentID: uuid.MustParse("00000000-0000-0000-0000-00000000000b"), CompanyID: "empresa-b"},
	}

	// Ajusta o ID do teste para bater com a rota mock
	remoteSegUUID := uuid.New()
	segments[1].SegmentID = remoteSegUUID
	// Cria servidor remoto que aceita qualquer UUID de segmento
	dynamicServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/internal/v1/segments/"+remoteSegUUID.String()+"/prepare" {
			prepareCalled = true
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "lamport_clock": 5})
			return
		}
		if r.URL.Path == "/internal/v1/segments/"+remoteSegUUID.String()+"/commit" {
			commitCalled = true
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
			return
		}
		http.NotFound(w, r)
	}))
	defer dynamicServer.Close()

	coord.peers.Add(Peer{CompanyID: "empresa-b", InternalURL: dynamicServer.URL})

	if err := coord.ConfirmItinerary(uuid.New(), segments); err != nil {
		t.Fatalf("2PC distribuído deveria suceder: %v", err)
	}

	if !prepareCalled || !commitCalled {
		t.Fatalf("fases do 2PC não chamadas no parceiro: prepare=%v commit=%v", prepareCalled, commitCalled)
	}

	if repo.rides[localRideID].Segments[0].AvailableSeats != 0 {
		t.Fatal("assento local deveria ser decrementado")
	}
}

func TestCoordinatorDistributedItineraryAbortOnConflict(t *testing.T) {
	abortCalled := false

	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Simula rejeição de assento indisponível (409 Conflict)
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "reason": "seat_unavailable"})
	}))
	defer remoteServer.Close()

	peers := []Peer{{CompanyID: "empresa-b", InternalURL: remoteServer.URL}}
	coord, repo, lockStore, _ := setupCoordinator(t, "empresa-a", peers)

	localRideID := uuid.New()
	localSegID := uuid.New()
	repo.rides[localRideID] = &models.Ride{
		ID: localRideID,
		Segments: []models.Stage{
			{ID: localSegID, AvailableSeats: 1},
		},
	}

	segments := []models.ReservedSegment{
		{RideID: localRideID, SegmentID: localSegID, CompanyID: "empresa-a"},
		{RideID: uuid.New(), SegmentID: uuid.New(), CompanyID: "empresa-b"},
	}

	err := coord.ConfirmItinerary(uuid.New(), segments)
	if err == nil {
		t.Fatal("deveria falhar quando parceiro rejeita com 409")
	}

	// Assento local NÃO pode ter sido decrementado (atomicidade garantida)
	if repo.rides[localRideID].Segments[0].AvailableSeats != 1 {
		t.Fatalf("assento local deveria permanecer 1 após abort, obtido %d", repo.rides[localRideID].Segments[0].AvailableSeats)
	}

	// Lock local deve ter sido desfeito
	if _, exists := lockStore.Get(localSegID.String()); exists {
		t.Fatal("lock local deveria ser liberado após o abort")
	}
	_ = abortCalled
}
