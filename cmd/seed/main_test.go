package main

import (
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
)

func TestSeedRidesCreateMultiDriverConnectionAtFeira(t *testing.T) {
	date := time.Date(2026, time.September, 17, 0, 0, 0, 0, time.Local)
	ana := anaRide(date)
	bruno := brunoRide(date)

	if len(ana.Segments) != 2 || len(bruno.Segments) != 2 {
		t.Fatalf("cada carona deveria possuir dois trechos: Ana=%d Bruno=%d", len(ana.Segments), len(bruno.Segments))
	}

	first := ana.Segments[0]
	connection := bruno.Segments[1]
	if first.Origin != enum.Alagoinhas || first.Destination != enum.FeiraDeSantana {
		t.Fatalf("primeiro trecho da conexão inesperado: %+v", first)
	}
	if connection.Origin != enum.FeiraDeSantana || connection.Destination != enum.LauroDeFreitas {
		t.Fatalf("segundo trecho da conexão inesperado: %+v", connection)
	}
	if connection.DepartureAt.Before(first.ArrivalAt) {
		t.Fatalf("conexão parte antes da chegada: chegada=%s partida=%s", first.ArrivalAt, connection.DepartureAt)
	}
	if !sameSeedRide(storedRideFromSeed(ana), ana) {
		t.Fatal("a mesma rota deveria ser reconhecida para manter o seed idempotente")
	}
}

func storedRideFromSeed(request protocol.CreateRideRequest) models.Ride {
	ride := models.Ride{DepartureAt: request.DepartureAt, Segments: make([]models.Stage, 0, len(request.Segments))}
	for _, stage := range request.Segments {
		ride.Segments = append(ride.Segments, models.Stage{
			Origin: stage.Origin, Destination: stage.Destination,
			DepartureAt: stage.DepartureAt, ArrivalAt: stage.ArrivalAt,
			PriceCents: stage.PriceCents, AvailableSeats: stage.AvailableSeats,
		})
	}
	return ride
}
