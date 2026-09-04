package server

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/google/uuid"
)

func TestRepositoryPersistsDriverVehicleAndRide(t *testing.T) {
	usersPath := filepath.Join(t.TempDir(), "data", "users.json")
	repository, err := NewRepository(usersPath)
	if err != nil {
		t.Fatalf("criar repositório: %v", err)
	}

	user := &models.User{
		ID:           uuid.New(),
		Name:         "Marina Souza",
		Email:        "marina@example.com",
		PasswordHash: "hash-de-teste",
		Role:         models.RoleDriver,
	}
	if err := repository.SaveUser(user); err != nil {
		t.Fatalf("salvar usuário: %v", err)
	}

	driver := &models.Driver{User: *user}
	driver.RegisterVehicle(models.Vehicle{Plate: "ABC-1234", Model: "Hatch", Color: "Azul", SeatCapacity: 4})
	if err := repository.SaveDriver(driver); err != nil {
		t.Fatalf("salvar motorista: %v", err)
	}

	ride := &models.Ride{
		ID:          uuid.New(),
		DriverID:    user.ID,
		DepartureAt: time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC),
		Segments: []models.Stage{
			{ID: uuid.New(), Origin: enum.Salvador, Destination: enum.FeiraDeSantana, PriceCents: 2500, AvailableSeats: 4},
		},
	}
	if err := repository.SaveRide(ride); err != nil {
		t.Fatalf("salvar carona: %v", err)
	}

	reloadedRepository, err := NewRepository(usersPath)
	if err != nil {
		t.Fatalf("recarregar repositório: %v", err)
	}

	reloadedDriver, err := reloadedRepository.GetDriverByUserID(user.ID)
	if err != nil || reloadedDriver.Vehicle == nil {
		t.Fatalf("motorista e veículo deveriam persistir: %v", err)
	}
	if reloadedDriver.Vehicle.Plate != "ABC-1234" {
		t.Errorf("placa carregada = %q, esperado ABC-1234", reloadedDriver.Vehicle.Plate)
	}

	reloadedRide, err := reloadedRepository.GetRideByID(ride.ID)
	if err != nil {
		t.Fatalf("carona deveria persistir: %v", err)
	}
	if reloadedRide.DriverID != user.ID || len(reloadedRide.Segments) != 1 {
		t.Errorf("carona carregada incorretamente: %+v", reloadedRide)
	}

	if err := reloadedRepository.CancelRide(ride.ID, user.ID); err != nil {
		t.Fatalf("cancelar carona: %v", err)
	}

	restartedRepository, err := NewRepository(usersPath)
	if err != nil {
		t.Fatalf("reiniciar repositório: %v", err)
	}

	cancelledRide, err := restartedRepository.GetRideByID(ride.ID)
	if err != nil || !cancelledRide.Cancelled {
		t.Fatalf("cancelamento deveria persistir: %v", err)
	}
}
