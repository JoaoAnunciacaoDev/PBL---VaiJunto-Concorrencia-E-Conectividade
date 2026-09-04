package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/google/uuid"
)

func TestPassengerSearchesDirectAndConnectedItineraries(t *testing.T) {
	server := newTestServer(t)
	passenger := protocol.CreateUserRequest{
		Name:     "Paula Passageira",
		Email:    "paula@example.com",
		Password: "Senha@123",
		Role:     models.RolePassenger,
	}
	if response := sendRequest(t, server, "register_user", passenger); response.Success != "success" {
		t.Fatalf("cadastro do passageiro deveria funcionar: %s", response.Message)
	}

	session := &Session{}
	if response := sendRequestWithSession(t, server, session, "login", protocol.LoginRequest{Email: passenger.Email, Password: passenger.Password}); response.Success != "success" {
		t.Fatalf("login do passageiro deveria funcionar: %s", response.Message)
	}

	date := time.Date(2026, time.September, 17, 0, 0, 0, 0, time.UTC)
	firstRide := itineraryTestRide(uuid.New(), time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC), enum.Salvador, enum.FeiraDeSantana, time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC), time.Date(2026, time.September, 17, 9, 30, 0, 0, time.UTC), 2500)
	connectingRide := itineraryTestRide(uuid.New(), time.Date(2026, time.September, 17, 9, 45, 0, 0, time.UTC), enum.FeiraDeSantana, enum.Jequie, time.Date(2026, time.September, 17, 9, 45, 0, 0, time.UTC), time.Date(2026, time.September, 17, 11, 30, 0, 0, time.UTC), 3000)
	directRide := itineraryTestRide(uuid.New(), time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC), enum.Salvador, enum.Jequie, time.Date(2026, time.September, 17, 10, 0, 0, 0, time.UTC), time.Date(2026, time.September, 17, 13, 0, 0, 0, time.UTC), 7000)
	tooSoonRide := itineraryTestRide(uuid.New(), time.Date(2026, time.September, 17, 9, 40, 0, 0, time.UTC), enum.FeiraDeSantana, enum.Jequie, time.Date(2026, time.September, 17, 9, 40, 0, 0, time.UTC), time.Date(2026, time.September, 17, 11, 20, 0, 0, time.UTC), 2000)

	for _, ride := range []*models.Ride{firstRide, connectingRide, directRide, tooSoonRide} {
		if err := server.repository.SaveRide(ride); err != nil {
			t.Fatalf("salvar carona de teste: %v", err)
		}
	}

	response := sendRequestWithSession(t, server, session, "search_itineraries", protocol.SearchItinerariesRequest{
		Origin: enum.Salvador, Destination: enum.Jequie, Date: date,
	})
	if response.Success != "success" {
		t.Fatalf("busca deveria funcionar: %s", response.Message)
	}

	var itineraries []protocol.ItineraryResponse
	if err := json.Unmarshal(response.Payload, &itineraries); err != nil {
		t.Fatalf("resposta deveria conter itinerários: %v", err)
	}

	foundDirect := false
	foundConnection := false
	for _, itinerary := range itineraries {
		if len(itinerary.Segments) == 1 && itinerary.Segments[0].RideID == directRide.ID {
			foundDirect = true
		}
		if len(itinerary.Segments) == 2 && itinerary.Segments[0].RideID == firstRide.ID && itinerary.Segments[1].RideID == connectingRide.ID {
			foundConnection = true
			if itinerary.TotalPriceCents != 5500 {
				t.Errorf("preço da conexão = %d, esperado 5500", itinerary.TotalPriceCents)
			}
		}
		for _, segment := range itinerary.Segments {
			if segment.RideID == tooSoonRide.ID {
				t.Fatal("conexão com intervalo menor que 15 minutos não deveria aparecer")
			}
		}
	}

	if !foundDirect {
		t.Fatal("itinerário direto deveria aparecer")
	}
	if !foundConnection {
		t.Fatal("itinerário com conexão válida deveria aparecer")
	}
}

func itineraryTestRide(driverID uuid.UUID, departureAt time.Time, origin, destination enum.City, stageDepartureAt, stageArrivalAt time.Time, priceCents int) *models.Ride {
	return &models.Ride{
		ID:          uuid.New(),
		DriverID:    driverID,
		DepartureAt: departureAt,
		Segments: []models.Stage{
			{
				ID:             uuid.New(),
				Origin:         origin,
				Destination:    destination,
				DepartureAt:    stageDepartureAt,
				ArrivalAt:      stageArrivalAt,
				PriceCents:     priceCents,
				AvailableSeats: 3,
			},
		},
	}
}
