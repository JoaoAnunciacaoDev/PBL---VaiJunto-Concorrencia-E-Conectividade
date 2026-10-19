package server

import (
	"errors"
	"fmt"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/google/uuid"
)

type storedDriver struct {
	UserID  uuid.UUID       `json:"user_id"`
	Vehicle *models.Vehicle `json:"vehicle,omitempty"`
}

func (r *Repository) loadDrivers() error {
	var drivers []storedDriver
	if err := readJSONFile(r.driversPath, &drivers); err != nil {
		return err
	}

	for _, stored := range drivers {
		if stored.UserID == uuid.Nil {
			return errors.New("arquivo de motoristas contém um registro inválido")
		}

		user, exists := r.userByIDLocked(stored.UserID)
		if !exists || !user.IsDriver() {
			return errors.New("arquivo de motoristas referencia um usuário inválido")
		}
		if _, exists := r.drivers[stored.UserID]; exists {
			return errors.New("arquivo de motoristas contém registros duplicados")
		}

		driver := &models.Driver{User: *user}
		if stored.Vehicle != nil {
			vehicleCopy := *stored.Vehicle
			driver.Vehicle = &vehicleCopy
		}
		r.drivers[stored.UserID] = driver
	}

	return nil
}

func (r *Repository) SaveDriver(driver *models.Driver) error {
	if driver == nil || driver.ID == uuid.Nil || !driver.IsDriver() {
		return errors.New("motorista inválido")
	}

	r.driversMu.Lock()
	defer r.driversMu.Unlock()

	oldDriver, existed := r.drivers[driver.ID]
	r.drivers[driver.ID] = cloneDriver(driver)
	if err := r.saveStateLocked(); err != nil {
		if existed {
			r.drivers[driver.ID] = oldDriver
		} else {
			delete(r.drivers, driver.ID)
		}
		return fmt.Errorf("salvar motorista: %w", err)
	}

	return nil
}

func (r *Repository) GetDriverByUserID(userID uuid.UUID) (*models.Driver, error) {
	r.driversMu.RLock()
	defer r.driversMu.RUnlock()

	driver, exists := r.drivers[userID]
	if !exists {
		return nil, errors.New("motorista não encontrado")
	}

	return cloneDriver(driver), nil
}

// UpdateDriverVehicle changes a vehicle while preserving the capacity required
// by the driver's active rides. It holds the repository lock for both the
// validation and the persistence, so a new ride cannot be published between
// those two steps.
func (r *Repository) UpdateDriverVehicle(driverID uuid.UUID, vehicle models.Vehicle) error {
	r.driversMu.Lock()
	defer r.driversMu.Unlock()

	r.ridesMu.RLock()
	defer r.ridesMu.RUnlock()

	r.reservationsMu.RLock()
	defer r.reservationsMu.RUnlock()

	driver, exists := r.drivers[driverID]
	if !exists || driver.Vehicle == nil {
		return errors.New("nenhum veículo cadastrado")
	}

	requiredCapacity := r.requiredVehicleCapacityForActiveRidesLocked(driverID)
	if vehicle.SeatCapacity < requiredCapacity {
		return fmt.Errorf("a capacidade não pode ser menor que %d, exigida pelas caronas ativas", requiredCapacity)
	}

	previousVehicle := *driver.Vehicle
	driver.Vehicle = &vehicle
	if err := r.saveStateLocked(); err != nil {
		driver.Vehicle = &previousVehicle
		return fmt.Errorf("salvar veículo: %w", err)
	}

	return nil
}

// RemoveDriverVehicle refuses to remove the vehicle while an active ride still
// belongs to the driver.
func (r *Repository) RemoveDriverVehicle(driverID uuid.UUID) error {
	r.driversMu.Lock()
	defer r.driversMu.Unlock()

	r.ridesMu.RLock()
	defer r.ridesMu.RUnlock()

	r.reservationsMu.RLock()
	defer r.reservationsMu.RUnlock()

	driver, exists := r.drivers[driverID]
	if !exists || driver.Vehicle == nil {
		return errors.New("nenhum veículo cadastrado")
	}

	if r.requiredVehicleCapacityForActiveRidesLocked(driverID) > 0 {
		return errors.New("não é possível remover o veículo enquanto houver caronas ativas")
	}

	previousVehicle := driver.Vehicle
	driver.Vehicle = nil
	if err := r.saveStateLocked(); err != nil {
		driver.Vehicle = previousVehicle
		return fmt.Errorf("remover veículo: %w", err)
	}

	return nil
}

func (r *Repository) requiredVehicleCapacityForActiveRidesLocked(driverID uuid.UUID) int {
	occupiedSeats := make(map[[2]uuid.UUID]int)
	for _, reservation := range r.reservations {
		if reservation.Status != enum.Confirmada {
			continue
		}
		for _, segment := range reservation.Segments {
			occupiedSeats[[2]uuid.UUID{segment.RideID, segment.SegmentID}]++
		}
	}

	requiredCapacity := 0
	for _, ride := range r.rides {
		if ride.DriverID != driverID || ride.Cancelled {
			continue
		}
		for _, stage := range ride.Segments {
			requiredCapacity = max(requiredCapacity, stage.AvailableSeats+occupiedSeats[[2]uuid.UUID{ride.ID, stage.ID}])
		}
	}

	return requiredCapacity
}
