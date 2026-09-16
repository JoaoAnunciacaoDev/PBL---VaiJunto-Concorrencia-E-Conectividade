package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
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
			{ID: uuid.New(), Origin: enum.Salvador, Destination: enum.FeiraDeSantana, DepartureAt: time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC), ArrivalAt: time.Date(2026, time.September, 17, 9, 30, 0, 0, time.UTC), PriceCents: 2500, AvailableSeats: 4},
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

func TestRepositoryKeepsPreviousStateWhenRenameIsInterrupted(t *testing.T) {
	usersPath := filepath.Join(t.TempDir(), "data", "users.json")
	repository, err := NewRepository(usersPath)
	if err != nil {
		t.Fatalf("criar repositório: %v", err)
	}

	passenger := &models.User{
		ID: uuid.New(), Name: "Passageiro", Email: "failure@example.com",
		PasswordHash: "hash", Role: models.RolePassenger,
	}
	if err := repository.SaveUser(passenger); err != nil {
		t.Fatalf("salvar passageiro: %v", err)
	}
	ride := reservationTestRide(1, 1)
	if err := repository.SaveRide(ride); err != nil {
		t.Fatalf("salvar carona: %v", err)
	}

	stateBefore, err := os.ReadFile(repository.statePath)
	if err != nil {
		t.Fatalf("ler estado anterior: %v", err)
	}
	repository.beforeStateRename = func() error { return errors.New("falha simulada antes do rename") }

	_, err = repository.ConfirmReservation(passenger.ID, []models.ReservedSegment{
		{RideID: ride.ID, SegmentID: ride.Segments[0].ID},
		{RideID: ride.ID, SegmentID: ride.Segments[1].ID},
	})
	if err == nil {
		t.Fatal("reserva deveria falhar antes do rename")
	}

	inMemoryRide, err := repository.GetRideByID(ride.ID)
	if err != nil {
		t.Fatalf("consultar carona em memória: %v", err)
	}
	if inMemoryRide.Segments[0].AvailableSeats != 1 || inMemoryRide.Segments[1].AvailableSeats != 1 {
		t.Fatalf("rollback em memória falhou: %+v", inMemoryRide.Segments)
	}
	if count := reservationCount(repository); count != 0 {
		t.Fatalf("reservas em memória = %d; esperado 0", count)
	}

	stateAfter, err := os.ReadFile(repository.statePath)
	if err != nil {
		t.Fatalf("ler estado após falha: %v", err)
	}
	if !bytes.Equal(stateBefore, stateAfter) {
		t.Fatal("state.json foi alterado apesar da interrupção antes do rename")
	}

	restarted, err := NewRepository(usersPath)
	if err != nil {
		t.Fatalf("reiniciar repositório: %v", err)
	}
	persistedRide, err := restarted.GetRideByID(ride.ID)
	if err != nil {
		t.Fatalf("consultar carona reiniciada: %v", err)
	}
	if persistedRide.Segments[0].AvailableSeats != 1 || persistedRide.Segments[1].AvailableSeats != 1 {
		t.Fatalf("estado reiniciado ficou parcial: %+v", persistedRide.Segments)
	}
	if count := reservationCount(restarted); count != 0 {
		t.Fatalf("reservas reiniciadas = %d; esperado 0", count)
	}
}

func TestRepositoryMigratesLegacyFilesToUnifiedState(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "data")
	usersPath := filepath.Join(dataDirectory, "users.json")
	if err := os.MkdirAll(dataDirectory, 0o755); err != nil {
		t.Fatalf("criar diretório: %v", err)
	}

	user := storedUser{
		ID: uuid.New(), Name: "Motorista Legado", Email: "legado@example.com",
		PasswordHash: "hash", Role: models.RoleDriver,
	}
	driver := storedDriver{
		UserID: user.ID,
		Vehicle: &models.Vehicle{
			Plate: "LEG-2026", Model: "Hatch", Color: "Azul", SeatCapacity: 2,
		},
	}
	ride := reservationTestRide(2, 2)
	ride.DriverID = user.ID

	writeLegacyJSON(t, usersPath, []storedUser{user})
	writeLegacyJSON(t, filepath.Join(dataDirectory, "drivers.json"), []storedDriver{driver})
	writeLegacyJSON(t, filepath.Join(dataDirectory, "rides.json"), []models.Ride{*ride})
	writeLegacyJSON(t, filepath.Join(dataDirectory, "reservations.json"), []models.Reservation{})

	repository, err := NewRepository(usersPath)
	if err != nil {
		t.Fatalf("migrar arquivos legados: %v", err)
	}
	if _, err := os.Stat(repository.statePath); err != nil {
		t.Fatalf("state.json deveria ser criado: %v", err)
	}
	if _, err := repository.GetRideByID(ride.ID); err != nil {
		t.Fatalf("carona migrada não encontrada: %v", err)
	}

	for _, legacyPath := range []string{
		usersPath,
		filepath.Join(dataDirectory, "drivers.json"),
		filepath.Join(dataDirectory, "rides.json"),
		filepath.Join(dataDirectory, "reservations.json"),
	} {
		if err := os.Remove(legacyPath); err != nil {
			t.Fatalf("remover arquivo legado de teste: %v", err)
		}
	}

	restarted, err := NewRepository(usersPath)
	if err != nil {
		t.Fatalf("reabrir somente state.json: %v", err)
	}
	if _, err := restarted.GetDriverByUserID(user.ID); err != nil {
		t.Fatalf("motorista migrado não encontrado: %v", err)
	}
	if _, err := restarted.GetRideByID(ride.ID); err != nil {
		t.Fatalf("carona migrada não encontrada após reinício: %v", err)
	}
}

func writeLegacyJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("serializar legado: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("gravar legado: %v", err)
	}
}
