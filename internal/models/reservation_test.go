package models

import (
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/google/uuid"
)

func TestReservationIsValid(t *testing.T) {
	reservation := Reservation{
		ID:          uuid.New(),
		PassengerID: uuid.New(),
		Status:      enum.Pendente,
		CreatedAt:   time.Now(),
		Segments: []ReservedSegment{
			{RideID: uuid.New(), SegmentID: uuid.New()},
			{RideID: uuid.New(), SegmentID: uuid.New()},
		},
	}

	if !reservation.IsValid() {
		t.Fatal("reserva com dados completos deveria ser válida")
	}
}

func TestReservationRejectsDuplicatedSegment(t *testing.T) {
	segment := ReservedSegment{RideID: uuid.New(), SegmentID: uuid.New()}
	reservation := Reservation{
		ID:          uuid.New(),
		PassengerID: uuid.New(),
		Status:      enum.Pendente,
		CreatedAt:   time.Now(),
		Segments:    []ReservedSegment{segment, segment},
	}

	if reservation.IsValid() {
		t.Fatal("reserva com trecho repetido deveria ser inválida")
	}
}
