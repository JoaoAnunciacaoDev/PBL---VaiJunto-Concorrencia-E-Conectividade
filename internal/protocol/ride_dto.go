package protocol

import (
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/google/uuid"
)

type CreateRideRequest struct {
	DepartureAt time.Time            `json:"departure_at"`
	Segments    []CreateStageRequest `json:"segments"`
}

type CreateStageRequest struct {
	Origin         enum.City `json:"origin"`
	Destination    enum.City `json:"destination"`
	DepartureAt    time.Time `json:"departure_at"`
	ArrivalAt      time.Time `json:"arrival_at"`
	PriceCents     int       `json:"price_cents"`
	AvailableSeats int       `json:"available_seats"`
}

type CancelRideRequest struct {
	RideID uuid.UUID `json:"ride_id"`
}
