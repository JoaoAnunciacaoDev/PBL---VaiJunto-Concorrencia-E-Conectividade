package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
)

func TestDriverCanCreateListAndCancelRide(t *testing.T) {
	server := newTestServer(t)
	driverRegistration := protocol.CreateUserRequest{
		Name:     "Maria Motorista",
		Email:    "maria.motorista@example.com",
		Password: "Senha@123",
		Role:     models.RoleDriver,
	}
	if response := sendRequest(t, server, "register_user", driverRegistration); response.Success != "success" {
		t.Fatalf("cadastro do motorista deveria funcionar: %s", response.Message)
	}

	session := &Session{}
	if response := sendRequestWithSession(t, server, session, "login", protocol.LoginRequest{
		Email: driverRegistration.Email, Password: driverRegistration.Password,
	}); response.Success != "success" {
		t.Fatalf("login do motorista deveria funcionar: %s", response.Message)
	}

	vehicle := protocol.CreateVehicleRequest{
		Plate:        "ABC-1234",
		Model:        "Hatch",
		Color:        "Azul",
		SeatCapacity: 4,
	}
	if response := sendRequestWithSession(t, server, session, "register_vehicle", vehicle); response.Success != "success" {
		t.Fatalf("cadastro do veículo deveria funcionar: %s", response.Message)
	}

	createRide := protocol.CreateRideRequest{
		DepartureAt: time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC),
		Segments: []protocol.CreateStageRequest{
			{Origin: enum.Salvador, Destination: enum.FeiraDeSantana, DepartureAt: time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC), ArrivalAt: time.Date(2026, time.September, 17, 9, 30, 0, 0, time.UTC), PriceCents: 2500, AvailableSeats: 4},
			{Origin: enum.FeiraDeSantana, Destination: enum.Jequie, DepartureAt: time.Date(2026, time.September, 17, 9, 45, 0, 0, time.UTC), ArrivalAt: time.Date(2026, time.September, 17, 11, 30, 0, 0, time.UTC), PriceCents: 3000, AvailableSeats: 3},
		},
	}
	createResponse := sendRequestWithSession(t, server, session, "create_ride", createRide)
	if createResponse.Success != "success" {
		t.Fatalf("publicação da carona deveria funcionar: %s", createResponse.Message)
	}

	var createdRide models.Ride
	if err := json.Unmarshal(createResponse.Payload, &createdRide); err != nil {
		t.Fatalf("resposta deveria conter a carona: %v", err)
	}
	if len(createdRide.Segments) != 2 || !createdRide.IsValid() {
		t.Fatalf("carona criada é inválida: %+v", createdRide)
	}

	listResponse := sendRequestWithSession(t, server, session, "list_my_rides", nil)
	if listResponse.Success != "success" {
		t.Fatalf("listagem deveria funcionar: %s", listResponse.Message)
	}

	var rides []models.Ride
	if err := json.Unmarshal(listResponse.Payload, &rides); err != nil {
		t.Fatalf("resposta deveria conter a lista de caronas: %v", err)
	}
	if len(rides) != 1 || rides[0].ID != createdRide.ID {
		t.Fatalf("lista de caronas incorreta: %+v", rides)
	}

	cancelResponse := sendRequestWithSession(t, server, session, "cancel_ride", protocol.CancelRideRequest{RideID: createdRide.ID})
	if cancelResponse.Success != "success" {
		t.Fatalf("cancelamento deveria funcionar: %s", cancelResponse.Message)
	}

	cancelledRide, err := server.repository.GetRideByID(createdRide.ID)
	if err != nil || !cancelledRide.Cancelled {
		t.Fatalf("carona deveria estar cancelada: %v", err)
	}
}

func TestCreateRideRejectsSeatsAboveVehicleCapacity(t *testing.T) {
	server := newTestServer(t)
	driver := protocol.CreateUserRequest{
		Name:     "Carlos Motorista",
		Email:    "carlos.motorista@example.com",
		Password: "Senha@123",
		Role:     models.RoleDriver,
	}
	if response := sendRequest(t, server, "register_user", driver); response.Success != "success" {
		t.Fatalf("cadastro do motorista deveria funcionar: %s", response.Message)
	}

	session := &Session{}
	if response := sendRequestWithSession(t, server, session, "login", protocol.LoginRequest{Email: driver.Email, Password: driver.Password}); response.Success != "success" {
		t.Fatalf("login deveria funcionar: %s", response.Message)
	}
	if response := sendRequestWithSession(t, server, session, "register_vehicle", protocol.CreateVehicleRequest{
		Plate: "DEF-5678", Model: "Sedan", Color: "Preto", SeatCapacity: 4,
	}); response.Success != "success" {
		t.Fatalf("cadastro do veículo deveria funcionar: %s", response.Message)
	}

	response := sendRequestWithSession(t, server, session, "create_ride", protocol.CreateRideRequest{
		DepartureAt: time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC),
		Segments: []protocol.CreateStageRequest{
			{Origin: enum.Salvador, Destination: enum.FeiraDeSantana, DepartureAt: time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC), ArrivalAt: time.Date(2026, time.September, 17, 9, 30, 0, 0, time.UTC), PriceCents: 2500, AvailableSeats: 5},
		},
	})
	if response.Success != "error" {
		t.Fatalf("carona com assentos acima da capacidade deveria falhar: %s", response.Message)
	}
}
