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
			{ID: uuid.New(), Origin: enum.Salvador, Destination: enum.FeiraDeSantana, DepartureAt: time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC), ArrivalAt: time.Date(2026, time.September, 17, 9, 30, 0, 0, time.UTC), PriceCents: 2500, AvailableSeats: 3},
			{ID: uuid.New(), Origin: enum.FeiraDeSantana, Destination: enum.Jequie, DepartureAt: time.Date(2026, time.September, 17, 9, 45, 0, 0, time.UTC), ArrivalAt: time.Date(2026, time.September, 17, 11, 30, 0, 0, time.UTC), PriceCents: 3000, AvailableSeats: 1},
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
		DepartureAt: time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC),
		Segments: []Stage{
			{ID: uuid.New(), Origin: enum.Salvador, Destination: enum.FeiraDeSantana, DepartureAt: time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC), ArrivalAt: time.Date(2026, time.September, 17, 9, 30, 0, 0, time.UTC), PriceCents: 2500, AvailableSeats: 3},
			{ID: uuid.New(), Origin: enum.Jequie, Destination: enum.VitoriaDaConquista, DepartureAt: time.Date(2026, time.September, 17, 9, 45, 0, 0, time.UTC), ArrivalAt: time.Date(2026, time.September, 17, 11, 30, 0, 0, time.UTC), PriceCents: 3000, AvailableSeats: 3},
		},
	}

	if ride.IsValid() {
		t.Fatal("carona com rota descontínua deveria ser inválida")
	}
}

func TestStageRequiresID(t *testing.T) {
	stage := Stage{
		Origin:         enum.Salvador,
		Destination:    enum.FeiraDeSantana,
		PriceCents:     2500,
		AvailableSeats: 3,
	}

	if stage.IsValid() {
		t.Fatal("trecho sem identificador deveria ser inválido")
	}
}
