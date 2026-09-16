package server

import (
	"fmt"
	"os"
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

	statePath        string
	usersPath        string
	driversPath      string
	ridesPath        string
	reservationsPath string

	// beforeStateRename permite que os testes simulem uma interrupção depois
	// que o novo estado foi sincronizado, mas antes de substituir state.json.
	beforeStateRename func() error
}

// NewRepository cria uma nova instância do repositório com o caminho para o arquivo de usuários fornecido.
// Ele inicializa os mapas de usuários, motoristas, caronas e reservas,
// e carrega os dados dos arquivos JSON correspondentes.
// Retorna um ponteiro para a instância do repositório e possíveis erros.
func NewRepository(usersPath string) (*Repository, error) {
	dataDirectory := filepath.Dir(usersPath)
	repository := &Repository{
		users:            make(map[string]*models.User),
		drivers:          make(map[uuid.UUID]*models.Driver),
		rides:            make(map[uuid.UUID]*models.Ride),
		reservations:     make(map[uuid.UUID]*models.Reservation),
		statePath:        filepath.Join(dataDirectory, "state.json"),
		usersPath:        usersPath,
		driversPath:      filepath.Join(dataDirectory, "drivers.json"),
		ridesPath:        filepath.Join(dataDirectory, "rides.json"),
		reservationsPath: filepath.Join(dataDirectory, "reservations.json"),
	}

	if _, err := os.Stat(repository.statePath); err == nil {
		if err := repository.loadPersistentState(); err != nil {
			return nil, fmt.Errorf("carregar estado: %w", err)
		}
		return repository, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("consultar estado: %w", err)
	}

	if err := repository.loadLegacyFiles(); err != nil {
		return nil, err
	}
	if repository.hasDataLocked() {
		if err := repository.saveStateLocked(); err != nil {
			return nil, fmt.Errorf("migrar arquivos legados: %w", err)
		}
	}

	return repository, nil
}

func (r *Repository) loadLegacyFiles() error {
	if err := r.loadUsers(); err != nil {
		return fmt.Errorf("carregar usuários: %w", err)
	}
	if err := r.loadDrivers(); err != nil {
		return fmt.Errorf("carregar motoristas: %w", err)
	}
	if err := r.loadRides(); err != nil {
		return fmt.Errorf("carregar caronas: %w", err)
	}
	if err := r.loadReservations(); err != nil {
		return fmt.Errorf("carregar reservas: %w", err)
	}
	return nil
}

func (r *Repository) hasDataLocked() bool {
	return len(r.users) > 0 || len(r.drivers) > 0 || len(r.rides) > 0 || len(r.reservations) > 0
}
