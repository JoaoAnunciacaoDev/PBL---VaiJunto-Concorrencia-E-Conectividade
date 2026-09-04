package protocol

import (
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
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

type GetRidePassengersRequest struct {
	RideID uuid.UUID `json:"ride_id"`
}

// RidePassengersResponse organiza os passageiros confirmados por trecho da
// carona. Apenas dados públicos do passageiro são enviados ao motorista.
type RidePassengersResponse struct {
	RideID   uuid.UUID                       `json:"ride_id"`
	Segments []RideSegmentPassengersResponse `json:"segments"`
}

type RideSegmentPassengersResponse struct {
	SegmentID   uuid.UUID      `json:"segment_id"`
	Origin      enum.City      `json:"origin"`
	Destination enum.City      `json:"destination"`
	DepartureAt time.Time      `json:"departure_at"`
	ArrivalAt   time.Time      `json:"arrival_at"`
	Passengers  []UserResponse `json:"passengers"`
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

type ConfirmReservationRequest struct {
	Segments []models.ReservedSegment `json:"segments"`
}

type CancelReservationRequest struct {
	ReservationID uuid.UUID `json:"reservation_id"`
}
