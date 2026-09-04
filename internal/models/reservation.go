package models

import (
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/google/uuid"
)

type Reservation struct {
	ID          uuid.UUID              `json:"id"`
	PassengerID uuid.UUID              `json:"passenger_id"`
	Status      enum.ReservationStatus `json:"status"`
	Segments    []ReservedSegment      `json:"segments"`
	CreatedAt   time.Time              `json:"created_at"`
}

func (r Reservation) IsValid() bool {
	if r.ID == uuid.Nil || r.PassengerID == uuid.Nil || !r.Status.IsValid() || r.CreatedAt.IsZero() || len(r.Segments) == 0 {
		return false
	}

	reservedSegments := make(map[ReservedSegment]struct{}, len(r.Segments))
	for _, segment := range r.Segments {
		if !segment.IsValid() {
			return false
		}

		if _, exists := reservedSegments[segment]; exists {
			return false
		}

		reservedSegments[segment] = struct{}{}
	}

	return true
}
