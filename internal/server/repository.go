package server

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/google/uuid"
)

type Repository struct {
	mu sync.RWMutex

	users        map[string]*models.User
	drivers      map[uuid.UUID]*models.Driver
	rides        map[uuid.UUID]*models.Ride
	reservations map[uuid.UUID]*models.Reservation

	usersPath        string
	driversPath      string
	ridesPath        string
	reservationsPath string
}

func NewRepository(usersPath string) (*Repository, error) {
	dataDirectory := filepath.Dir(usersPath)
	repository := &Repository{
		users:            make(map[string]*models.User),
		drivers:          make(map[uuid.UUID]*models.Driver),
		rides:            make(map[uuid.UUID]*models.Ride),
		reservations:     make(map[uuid.UUID]*models.Reservation),
		usersPath:        usersPath,
		driversPath:      filepath.Join(dataDirectory, "drivers.json"),
		ridesPath:        filepath.Join(dataDirectory, "rides.json"),
		reservationsPath: filepath.Join(dataDirectory, "reservations.json"),
	}

	if err := repository.loadUsers(); err != nil {
		return nil, fmt.Errorf("carregar usuários: %w", err)
	}
	if err := repository.loadDrivers(); err != nil {
		return nil, fmt.Errorf("carregar motoristas: %w", err)
	}
	if err := repository.loadRides(); err != nil {
		return nil, fmt.Errorf("carregar caronas: %w", err)
	}
	if err := repository.loadReservations(); err != nil {
		return nil, fmt.Errorf("carregar reservas: %w", err)
	}

	return repository, nil
}
