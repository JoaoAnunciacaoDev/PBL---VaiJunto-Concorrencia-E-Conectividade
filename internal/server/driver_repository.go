package server

import (
	"errors"
	"fmt"
	"sort"

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
		if stored.UserID == uuid.Nil || stored.Vehicle == nil {
			return errors.New("arquivo de motoristas contém um registro inválido")
		}

		user, exists := r.userByIDLocked(stored.UserID)
		if !exists || !user.IsDriver() {
			return errors.New("arquivo de motoristas referencia um usuário inválido")
		}
		if _, exists := r.drivers[stored.UserID]; exists {
			return errors.New("arquivo de motoristas contém registros duplicados")
		}

		vehicleCopy := *stored.Vehicle
		r.drivers[stored.UserID] = &models.Driver{
			User:    *user,
			Vehicle: &vehicleCopy,
		}
	}

	return nil
}

func (r *Repository) saveDriversLocked() error {
	drivers := make([]storedDriver, 0, len(r.drivers))
	for _, driver := range r.drivers {
		var vehicle *models.Vehicle
		if driver.Vehicle != nil {
			vehicleCopy := *driver.Vehicle
			vehicle = &vehicleCopy
		}

		drivers = append(drivers, storedDriver{
			UserID:  driver.ID,
			Vehicle: vehicle,
		})
	}

	sort.Slice(drivers, func(i, j int) bool {
		return drivers[i].UserID.String() < drivers[j].UserID.String()
	})

	return writeJSONFileAtomic(r.driversPath, ".drivers-*.tmp", drivers)
}

func (r *Repository) SaveDriver(driver *models.Driver) error {
	if driver == nil || driver.ID == uuid.Nil || !driver.IsDriver() {
		return errors.New("motorista inválido")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	oldDriver, existed := r.drivers[driver.ID]
	r.drivers[driver.ID] = driver
	if err := r.saveDriversLocked(); err != nil {
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
	r.mu.RLock()
	defer r.mu.RUnlock()

	driver, exists := r.drivers[userID]
	if !exists {
		return nil, errors.New("motorista não encontrado")
	}

	return driver, nil
}
