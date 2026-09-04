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

type SearchItinerariesRequest struct {
	Origin      enum.City `json:"origin"`
	Destination enum.City `json:"destination"`
	Date        time.Time `json:"date"`
}

type ItineraryResponse struct {
	Segments        []ItinerarySegmentResponse `json:"segments"`
	DepartureAt     time.Time                  `json:"departure_at"`
	ArrivalAt       time.Time                  `json:"arrival_at"`
	TotalPriceCents int                        `json:"total_price_cents"`
}

type ItinerarySegmentResponse struct {
	RideID         uuid.UUID `json:"ride_id"`
	SegmentID      uuid.UUID `json:"segment_id"`
	Origin         enum.City `json:"origin"`
	Destination    enum.City `json:"destination"`
	DepartureAt    time.Time `json:"departure_at"`
	ArrivalAt      time.Time `json:"arrival_at"`
	PriceCents     int       `json:"price_cents"`
	AvailableSeats int       `json:"available_seats"`
}
