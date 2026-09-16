package server

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/google/uuid"
)

const persistentStateVersion = 1

// PersistentState reúne todas as estruturas que precisam mudar de forma
// atômica. Uma reserva confirmada e os assentos correspondentes passam a ser
// gravados no mesmo arquivo e no mesmo rename.
type PersistentState struct {
	Version      int                  `json:"version"`
	Users        []storedUser         `json:"users"`
	Drivers      []storedDriver       `json:"drivers"`
	Rides        []models.Ride        `json:"rides"`
	Reservations []models.Reservation `json:"reservations"`
}

func (r *Repository) saveStateLocked() error {
	state := r.persistentStateLocked()
	return writeJSONFileAtomic(r.statePath, ".state-*.tmp", state, r.beforeStateRename)
}

func (r *Repository) persistentStateLocked() PersistentState {
	state := PersistentState{Version: persistentStateVersion}

	state.Users = make([]storedUser, 0, len(r.users))
	for _, user := range r.users {
		state.Users = append(state.Users, storedUser{
			ID: user.ID, Name: user.Name, Email: user.Email,
			PasswordHash: user.PasswordHash, Role: user.Role,
		})
	}
	sort.Slice(state.Users, func(i, j int) bool { return state.Users[i].Email < state.Users[j].Email })

	state.Drivers = make([]storedDriver, 0, len(r.drivers))
	for _, driver := range r.drivers {
		var vehicle *models.Vehicle
		if driver.Vehicle != nil {
			copy := *driver.Vehicle
			vehicle = &copy
		}
		state.Drivers = append(state.Drivers, storedDriver{UserID: driver.ID, Vehicle: vehicle})
	}
	sort.Slice(state.Drivers, func(i, j int) bool {
		return state.Drivers[i].UserID.String() < state.Drivers[j].UserID.String()
	})

	state.Rides = make([]models.Ride, 0, len(r.rides))
	for _, ride := range r.rides {
		state.Rides = append(state.Rides, *cloneRide(ride))
	}
	sort.Slice(state.Rides, func(i, j int) bool { return state.Rides[i].ID.String() < state.Rides[j].ID.String() })

	state.Reservations = make([]models.Reservation, 0, len(r.reservations))
	for _, reservation := range r.reservations {
		state.Reservations = append(state.Reservations, *cloneReservation(reservation))
	}
	sort.Slice(state.Reservations, func(i, j int) bool {
		return state.Reservations[i].CreatedAt.Before(state.Reservations[j].CreatedAt)
	})

	return state
}

func (r *Repository) loadPersistentState() error {
	var state PersistentState
	if err := readJSONFile(r.statePath, &state); err != nil {
		return err
	}
	if state.Version != persistentStateVersion {
		return fmt.Errorf("versão de estado não suportada: %d", state.Version)
	}

	for _, stored := range state.Users {
		if stored.ID == uuid.Nil || stored.Email == "" || stored.PasswordHash == "" ||
			(stored.Role != models.RolePassenger && stored.Role != models.RoleDriver) {
			return errors.New("estado contém usuário inválido")
		}
		email := strings.ToLower(strings.TrimSpace(stored.Email))
		if _, exists := r.users[email]; exists {
			return errors.New("estado contém e-mails duplicados")
		}
		r.users[email] = &models.User{
			ID: stored.ID, Name: stored.Name, Email: email,
			PasswordHash: stored.PasswordHash, Role: stored.Role,
		}
	}

	for _, stored := range state.Drivers {
		if stored.UserID == uuid.Nil {
			return errors.New("estado contém motorista inválido")
		}
		user, exists := r.userByIDLocked(stored.UserID)
		if !exists || !user.IsDriver() {
			return errors.New("estado contém motorista sem usuário válido")
		}
		if _, exists := r.drivers[stored.UserID]; exists {
			return errors.New("estado contém motoristas duplicados")
		}
		driver := &models.Driver{User: *user}
		if stored.Vehicle != nil {
			copy := *stored.Vehicle
			driver.Vehicle = &copy
		}
		r.drivers[stored.UserID] = driver
	}

	for _, ride := range state.Rides {
		if !ride.IsValid() {
			return errors.New("estado contém carona inválida")
		}
		if _, exists := r.rides[ride.ID]; exists {
			return errors.New("estado contém caronas duplicadas")
		}
		r.rides[ride.ID] = cloneRide(&ride)
	}

	for _, reservation := range state.Reservations {
		if !reservation.IsValid() {
			return errors.New("estado contém reserva inválida")
		}
		if _, exists := r.reservations[reservation.ID]; exists {
			return errors.New("estado contém reservas duplicadas")
		}
		r.reservations[reservation.ID] = cloneReservation(&reservation)
	}

	return nil
}
