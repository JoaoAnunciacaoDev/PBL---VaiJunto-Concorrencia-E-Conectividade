package models

import (
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/google/uuid"
)

func TestRideIsValidWithContinuousSegments(t *testing.T) {
	ride := Ride{
		ID:          uuid.New(),
		DriverID:    uuid.New(),
		DepartureAt: time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC),
		Segments: []Stage{
			{Origin: enum.Salvador, Destination: enum.FeiraDeSantana, PriceCents: 2500, AvailableSeats: 3},
			{Origin: enum.FeiraDeSantana, Destination: enum.Jequie, PriceCents: 3000, AvailableSeats: 1},
		},
	}

	if !ride.IsValid() {
		t.Fatal("carona com rota contínua deveria ser válida")
	}

	if got := ride.TotalPriceCents(); got != 5500 {
		t.Errorf("preço total = %d, esperado 5500", got)
	}

	if got := ride.MinimumAvailableSeats(); got != 1 {
		t.Errorf("menor disponibilidade = %d, esperado 1", got)
	}
}

func TestRideRejectsDiscontinuousSegments(t *testing.T) {
	ride := Ride{
		ID:          uuid.New(),
		DriverID:    uuid.New(),
		DepartureAt: time.Now(),
		Segments: []Stage{
			{Origin: enum.Salvador, Destination: enum.FeiraDeSantana, PriceCents: 2500, AvailableSeats: 3},
			{Origin: enum.Jequie, Destination: enum.VitoriaDaConquista, PriceCents: 3000, AvailableSeats: 3},
		},
	}

	if ride.IsValid() {
		t.Fatal("carona com rota descontínua deveria ser inválida")
	}
}
