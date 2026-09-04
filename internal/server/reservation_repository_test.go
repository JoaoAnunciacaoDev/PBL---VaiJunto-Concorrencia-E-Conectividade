package server

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/google/uuid"
)

func TestConcurrentReservationsDoNotSellSameSeatTwice(t *testing.T) {
	repository, err := NewRepository(filepath.Join(t.TempDir(), "data", "users.json"))
	if err != nil {
		t.Fatalf("criar repositório: %v", err)
	}

	passengers := []*models.User{
		{ID: uuid.New(), Name: "Ana", Email: "ana@example.com", PasswordHash: "hash", Role: models.RolePassenger},
		{ID: uuid.New(), Name: "Bia", Email: "bia@example.com", PasswordHash: "hash", Role: models.RolePassenger},
	}
	for _, passenger := range passengers {
		if err := repository.SaveUser(passenger); err != nil {
			t.Fatalf("salvar passageiro: %v", err)
		}
	}

	ride := reservationTestRide(1, 1)
	if err := repository.SaveRide(ride); err != nil {
		t.Fatalf("salvar carona: %v", err)
	}

	segments := []models.ReservedSegment{{RideID: ride.ID, SegmentID: ride.Segments[0].ID}}
	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	results := make(chan error, len(passengers))
	for _, passenger := range passengers {
		waitGroup.Add(1)
		go func(passengerID uuid.UUID) {
			defer waitGroup.Done()
			<-start
			_, err := repository.ConfirmReservation(passengerID, segments)
			results <- err
		}(passenger.ID)
	}

	close(start)
	waitGroup.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("reservas confirmadas = %d, esperado 1", successes)
	}

	storedRide, err := repository.GetRideByID(ride.ID)
	if err != nil || storedRide.Segments[0].AvailableSeats != 0 {
		t.Fatalf("assentos restantes incorretos: %v", err)
	}
}

func TestReservationDoesNotPartiallyReserveItinerary(t *testing.T) {
	repository, err := NewRepository(filepath.Join(t.TempDir(), "data", "users.json"))
	if err != nil {
		t.Fatalf("criar repositório: %v", err)
	}

	passenger := &models.User{ID: uuid.New(), Name: "Caio", Email: "caio@example.com", PasswordHash: "hash", Role: models.RolePassenger}
	if err := repository.SaveUser(passenger); err != nil {
		t.Fatalf("salvar passageiro: %v", err)
	}

	ride := reservationTestRide(1, 0)
	if err := repository.SaveRide(ride); err != nil {
		t.Fatalf("salvar carona: %v", err)
	}

	_, err = repository.ConfirmReservation(passenger.ID, []models.ReservedSegment{
		{RideID: ride.ID, SegmentID: ride.Segments[0].ID},
		{RideID: ride.ID, SegmentID: ride.Segments[1].ID},
	})
	if err == nil {
		t.Fatal("reserva com trecho sem assento deveria falhar")
	}

	storedRide, err := repository.GetRideByID(ride.ID)
	if err != nil {
		t.Fatalf("consultar carona: %v", err)
	}
	if storedRide.Segments[0].AvailableSeats != 1 || storedRide.Segments[1].AvailableSeats != 0 {
		t.Fatalf("assentos foram alterados parcialmente: %+v", storedRide.Segments)
	}
}

func TestCancelRideCancelsRelatedReservationsAndRestoresSeats(t *testing.T) {
	usersPath := filepath.Join(t.TempDir(), "data", "users.json")
	repository, err := NewRepository(usersPath)
	if err != nil {
		t.Fatalf("criar repositório: %v", err)
	}

	passenger := &models.User{ID: uuid.New(), Name: "Davi", Email: "davi@example.com", PasswordHash: "hash", Role: models.RolePassenger}
	if err := repository.SaveUser(passenger); err != nil {
		t.Fatalf("salvar passageiro: %v", err)
	}

	ride := reservationTestRide(1, 1)
	if err := repository.SaveRide(ride); err != nil {
		t.Fatalf("salvar carona: %v", err)
	}
	reservation, err := repository.ConfirmReservation(passenger.ID, []models.ReservedSegment{
		{RideID: ride.ID, SegmentID: ride.Segments[0].ID},
		{RideID: ride.ID, SegmentID: ride.Segments[1].ID},
	})
	if err != nil {
		t.Fatalf("confirmar reserva: %v", err)
	}

	if err := repository.CancelRide(ride.ID, ride.DriverID); err != nil {
		t.Fatalf("cancelar carona: %v", err)
	}

	reservations, err := repository.GetReservationsByPassengerID(passenger.ID)
	if err != nil || len(reservations) != 1 || reservations[0].ID != reservation.ID {
		t.Fatalf("reserva deveria permanecer consultável: %v", err)
	}
	if reservations[0].Status != enum.Cancelada {
		t.Fatalf("status da reserva = %s, esperado cancelada", reservations[0].Status)
	}

	cancelledRide, err := repository.GetRideByID(ride.ID)
	if err != nil || !cancelledRide.Cancelled {
		t.Fatalf("carona deveria estar cancelada: %v", err)
	}
	if cancelledRide.Segments[0].AvailableSeats != 1 || cancelledRide.Segments[1].AvailableSeats != 1 {
		t.Fatalf("assentos deveriam ser devolvidos: %+v", cancelledRide.Segments)
	}

	restartedRepository, err := NewRepository(usersPath)
	if err != nil {
		t.Fatalf("reiniciar repositório: %v", err)
	}
	persistedReservations, err := restartedRepository.GetReservationsByPassengerID(passenger.ID)
	if err != nil || len(persistedReservations) != 1 || persistedReservations[0].Status != enum.Cancelada {
		t.Fatalf("cancelamento da reserva deveria persistir: %v", err)
	}
}

func reservationTestRide(firstSeats, secondSeats int) *models.Ride {
	departure := time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC)
	return &models.Ride{
		ID:          uuid.New(),
		DriverID:    uuid.New(),
		DepartureAt: departure,
		Segments: []models.Stage{
			{ID: uuid.New(), Origin: enum.Salvador, Destination: enum.FeiraDeSantana, DepartureAt: departure, ArrivalAt: departure.Add(90 * time.Minute), PriceCents: 2500, AvailableSeats: firstSeats},
			{ID: uuid.New(), Origin: enum.FeiraDeSantana, Destination: enum.Jequie, DepartureAt: departure.Add(105 * time.Minute), ArrivalAt: departure.Add(210 * time.Minute), PriceCents: 3000, AvailableSeats: secondSeats},
		},
	}
}
