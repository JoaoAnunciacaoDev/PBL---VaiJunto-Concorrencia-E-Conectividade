package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
)

func TestVehicleCannotInvalidateActiveRide(t *testing.T) {
	server := newTestServer(t)
	driver := protocol.CreateUserRequest{
		Name:     "Lia Motorista",
		Email:    "lia.motorista@example.com",
		Password: "Senha@123",
		Role:     models.RoleDriver,
	}
	if response := sendRequest(t, server, "register_user", driver); response.Success != "success" {
		t.Fatalf("cadastro do motorista: %s", response.Message)
	}

	session := &Session{}
	if response := sendRequestWithSession(t, server, session, "login", protocol.LoginRequest{Email: driver.Email, Password: driver.Password}); response.Success != "success" {
		t.Fatalf("login do motorista: %s", response.Message)
	}

	if response := sendRequestWithSession(t, server, session, "register_vehicle", protocol.CreateVehicleRequest{
		Plate: "ABC-1234", Model: "Hatch", Color: "Azul", SeatCapacity: 4,
	}); response.Success != "success" {
		t.Fatalf("cadastro do veículo: %s", response.Message)
	}

	departure := time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC)
	createResponse := sendRequestWithSession(t, server, session, "create_ride", protocol.CreateRideRequest{
		DepartureAt: departure,
		Segments: []protocol.CreateStageRequest{{
			Origin: enum.Salvador, Destination: enum.FeiraDeSantana,
			DepartureAt: departure, ArrivalAt: departure.Add(90 * time.Minute),
			PriceCents: 2500, AvailableSeats: 4,
		}},
	})
	if createResponse.Success != "success" {
		t.Fatalf("publicação da carona: %s", createResponse.Message)
	}

	if response := sendRequestWithSession(t, server, session, "update_vehicle", protocol.CreateVehicleRequest{
		Plate: "ABC-1234", Model: "Hatch", Color: "Azul", SeatCapacity: 3,
	}); response.Success != "error" {
		t.Fatalf("reduzir capacidade abaixo da carona ativa deveria falhar: %s", response.Message)
	}

	vehicleResponse := sendRequestWithSession(t, server, session, "get_my_vehicle", nil)
	if vehicleResponse.Success != "success" {
		t.Fatalf("consulta do veículo: %s", vehicleResponse.Message)
	}
	var vehicle models.Vehicle
	if err := json.Unmarshal(vehicleResponse.Payload, &vehicle); err != nil {
		t.Fatalf("ler veículo: %v", err)
	}
	if vehicle.SeatCapacity != 4 {
		t.Fatalf("capacidade foi alterada apesar da recusa: %d", vehicle.SeatCapacity)
	}

	if response := sendRequestWithSession(t, server, session, "remove_vehicle", nil); response.Success != "error" {
		t.Fatalf("remover veículo com carona ativa deveria falhar: %s", response.Message)
	}

	if response := sendRequestWithSession(t, server, session, "update_vehicle", protocol.CreateVehicleRequest{
		Plate: "ABC-1234", Model: "Hatch", Color: "Azul", SeatCapacity: 5,
	}); response.Success != "success" {
		t.Fatalf("aumentar capacidade deveria funcionar: %s", response.Message)
	}

	var ride models.Ride
	if err := json.Unmarshal(createResponse.Payload, &ride); err != nil {
		t.Fatalf("ler carona: %v", err)
	}
	if response := sendRequestWithSession(t, server, session, "cancel_ride", protocol.CancelRideRequest{RideID: ride.ID}); response.Success != "success" {
		t.Fatalf("cancelar carona: %s", response.Message)
	}
	if response := sendRequestWithSession(t, server, session, "remove_vehicle", nil); response.Success != "success" {
		t.Fatalf("remover veículo após cancelar carona deveria funcionar: %s", response.Message)
	}
}
