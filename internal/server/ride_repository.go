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

func (r *Repository) loadRides() error {
	var rides []models.Ride
	if err := readJSONFile(r.ridesPath, &rides); err != nil {
		return err
	}

	for _, ride := range rides {
		if !ride.IsValid() {
			return errors.New("arquivo de caronas contém um registro inválido")
		}
		if _, exists := r.rides[ride.ID]; exists {
			return errors.New("arquivo de caronas contém identificadores duplicados")
		}

		rideCopy := ride
		r.rides[ride.ID] = &rideCopy
	}

	return nil
}

func (r *Repository) saveRidesLocked() error {
	rides := make([]models.Ride, 0, len(r.rides))
	for _, ride := range r.rides {
		rides = append(rides, *ride)
	}

	sort.Slice(rides, func(i, j int) bool {
		return rides[i].ID.String() < rides[j].ID.String()
	})

	return writeJSONFileAtomic(r.ridesPath, ".rides-*.tmp", rides)
}

func (r *Repository) SaveRide(ride *models.Ride) error {
	if ride == nil || !ride.IsValid() {
		return errors.New("carona inválida")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	oldRide, existed := r.rides[ride.ID]
	r.rides[ride.ID] = ride
	if err := r.saveRidesLocked(); err != nil {
		if existed {
			r.rides[ride.ID] = oldRide
		} else {
			delete(r.rides, ride.ID)
		}
		return fmt.Errorf("salvar carona: %w", err)
	}

	return nil
}

func (r *Repository) GetRideByID(id uuid.UUID) (*models.Ride, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ride, exists := r.rides[id]
	if !exists {
		return nil, errors.New("carona não encontrada")
	}

	return ride, nil
}

func (r *Repository) GetRidesByDriverID(driverID uuid.UUID) ([]*models.Ride, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rides := make([]*models.Ride, 0)
	for _, ride := range r.rides {
		if ride.DriverID == driverID {
			rides = append(rides, ride)
		}
	}

	sort.Slice(rides, func(i, j int) bool {
		return rides[i].DepartureAt.Before(rides[j].DepartureAt)
	})

	return rides, nil
}

func (r *Repository) GetActiveRidesByDate(date time.Time) ([]*models.Ride, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rides := make([]*models.Ride, 0)
	for _, ride := range r.rides {
		if !ride.Cancelled && sameCalendarDate(ride.DepartureAt, date) {
			rides = append(rides, ride)
		}
	}

	sort.Slice(rides, func(i, j int) bool {
		return rides[i].DepartureAt.Before(rides[j].DepartureAt)
	})

	return rides, nil
}

func (r *Repository) CancelRide(rideID, driverID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ride, exists := r.rides[rideID]
	if !exists {
		return errors.New("carona não encontrada")
	}
	if ride.DriverID != driverID {
		return errors.New("motorista não pode cancelar esta carona")
	}
	if ride.Cancelled {
		return errors.New("carona já está cancelada")
	}

	affectedReservations, err := r.confirmedReservationsForRideLocked(rideID)
	if err != nil {
		return fmt.Errorf("cancelar reservas da carona: %w", err)
	}

	ride.Cancel()
	for _, affected := range affectedReservations {
		affected.reservation.Status = enum.Cancelada
		for _, stage := range affected.stages {
			stage.stage.AvailableSeats++
		}
	}

	if err := r.saveRidesLocked(); err != nil {
		r.restoreRideCancellation(ride, affectedReservations)
		return fmt.Errorf("cancelar carona: %w", err)
	}
	if err := r.saveReservationsLocked(); err != nil {
		r.restoreRideCancellation(ride, affectedReservations)
		// A escrita de cada arquivo é atômica, mas rides.json já foi salvo. Esta
		// segunda escrita restaura o arquivo para o mesmo estado da memória.
		if restoreErr := r.saveRidesLocked(); restoreErr != nil {
			return fmt.Errorf("cancelar carona e restaurar estado: %w", restoreErr)
		}
		return fmt.Errorf("salvar cancelamento das reservas: %w", err)
	}

	return nil
}

func sameCalendarDate(first, second time.Time) bool {
	firstYear, firstMonth, firstDay := first.Date()
	secondYear, secondMonth, secondDay := second.Date()
	return firstYear == secondYear && firstMonth == secondMonth && firstDay == secondDay
}
