package server

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/google/uuid"
)

func (r *Repository) loadReservations() error {
	var reservations []models.Reservation
	if err := readJSONFile(r.reservationsPath, &reservations); err != nil {
		return err
	}

	for _, reservation := range reservations {
		if !reservation.IsValid() {
			return errors.New("arquivo de reservas contém um registro inválido")
		}
		if _, exists := r.reservations[reservation.ID]; exists {
			return errors.New("arquivo de reservas contém identificadores duplicados")
		}

		reservationCopy := reservation
		r.reservations[reservation.ID] = &reservationCopy
	}

	return nil
}

func (r *Repository) saveReservationsLocked() error {
	reservations := make([]models.Reservation, 0, len(r.reservations))
	for _, reservation := range r.reservations {
		reservations = append(reservations, *reservation)
	}

	sort.Slice(reservations, func(i, j int) bool {
		return reservations[i].CreatedAt.Before(reservations[j].CreatedAt)
	})

	return writeJSONFileAtomic(r.reservationsPath, ".reservations-*.tmp", reservations)
}

func (r *Repository) ConfirmReservation(passengerID uuid.UUID, segments []models.ReservedSegment) (*models.Reservation, error) {
	if passengerID == uuid.Nil || len(segments) == 0 {
		return nil, errors.New("reserva inválida")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if user, exists := r.userByIDLocked(passengerID); !exists || user.Role != models.RolePassenger {
		return nil, errors.New("passageiro inválido")
	}

	stages, err := r.reservedStagesLocked(segments)
	if err != nil {
		return nil, err
	}
	if !reservedStagesFormItinerary(stages) {
		return nil, errors.New("trechos não formam um itinerário válido")
	}

	for _, stage := range stages {
		if stage.stage.AvailableSeats <= 0 {
			return nil, errors.New("não há assentos disponíveis em todos os trechos")
		}
	}

	for _, stage := range stages {
		stage.stage.AvailableSeats--
	}

	reservation := &models.Reservation{
		ID:          uuid.New(),
		PassengerID: passengerID,
		Status:      enum.Confirmada,
		Segments:    append([]models.ReservedSegment(nil), segments...),
		CreatedAt:   time.Now(),
	}
	r.reservations[reservation.ID] = reservation

	if err := r.saveRidesLocked(); err != nil {
		r.restoreSeats(stages)
		delete(r.reservations, reservation.ID)
		return nil, fmt.Errorf("salvar assentos: %w", err)
	}
	if err := r.saveReservationsLocked(); err != nil {
		r.restoreSeats(stages)
		delete(r.reservations, reservation.ID)
		return nil, fmt.Errorf("salvar reserva: %w", err)
	}

	return reservation, nil
}

func (r *Repository) GetReservationsByPassengerID(passengerID uuid.UUID) ([]*models.Reservation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	reservations := make([]*models.Reservation, 0)
	for _, reservation := range r.reservations {
		if reservation.PassengerID == passengerID {
			reservations = append(reservations, reservation)
		}
	}

	sort.Slice(reservations, func(i, j int) bool {
		return reservations[i].CreatedAt.After(reservations[j].CreatedAt)
	})
	return reservations, nil
}

func (r *Repository) CancelReservation(reservationID, passengerID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	reservation, exists := r.reservations[reservationID]
	if !exists {
		return errors.New("reserva não encontrada")
	}
	if reservation.PassengerID != passengerID {
		return errors.New("passageiro não pode cancelar esta reserva")
	}
	if reservation.Status != enum.Confirmada {
		return errors.New("reserva não pode ser cancelada")
	}

	stages, err := r.reservedStagesLocked(reservation.Segments)
	if err != nil {
		return fmt.Errorf("reserva possui trechos indisponíveis: %w", err)
	}
	for _, stage := range stages {
		stage.stage.AvailableSeats++
	}

	reservation.Status = enum.Cancelada
	if err := r.saveRidesLocked(); err != nil {
		r.restoreSeats(stages)
		reservation.Status = enum.Confirmada
		return fmt.Errorf("devolver assentos: %w", err)
	}
	if err := r.saveReservationsLocked(); err != nil {
		r.restoreSeats(stages)
		reservation.Status = enum.Confirmada
		return fmt.Errorf("salvar cancelamento: %w", err)
	}

	return nil
}

type reservedStage struct {
	rideID uuid.UUID
	stage  *models.Stage
}

func (r *Repository) reservedStagesLocked(segments []models.ReservedSegment) ([]reservedStage, error) {
	stages := make([]reservedStage, 0, len(segments))
	seen := make(map[models.ReservedSegment]struct{}, len(segments))
	for _, segment := range segments {
		if !segment.IsValid() {
			return nil, errors.New("trecho reservado inválido")
		}
		if _, exists := seen[segment]; exists {
			return nil, errors.New("trecho reservado repetido")
		}
		seen[segment] = struct{}{}

		ride, exists := r.rides[segment.RideID]
		if !exists || ride.Cancelled {
			return nil, errors.New("carona indisponível")
		}

		found := false
		for index := range ride.Segments {
			if ride.Segments[index].ID == segment.SegmentID {
				stages = append(stages, reservedStage{rideID: ride.ID, stage: &ride.Segments[index]})
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("trecho não encontrado")
		}
	}

	return stages, nil
}

func reservedStagesFormItinerary(stages []reservedStage) bool {
	for index, stage := range stages {
		if !stage.stage.IsValid() {
			return false
		}
		if index == 0 {
			continue
		}

		previous := stages[index-1]
		if previous.stage.Destination != stage.stage.Origin {
			return false
		}

		minimumDeparture := previous.stage.ArrivalAt
		if previous.rideID != stage.rideID {
			minimumDeparture = minimumDeparture.Add(minimumConnectionTime)
		}
		if stage.stage.DepartureAt.Before(minimumDeparture) {
			return false
		}
	}

	return len(stages) > 0
}

func (r *Repository) restoreSeats(stages []reservedStage) {
	for _, stage := range stages {
		stage.stage.AvailableSeats++
	}
}
