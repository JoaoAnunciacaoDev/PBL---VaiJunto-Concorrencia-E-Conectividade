package protocol

import (
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/google/uuid"
)

// ReservationResponse é a visão de uma reserva devolvida ao passageiro. Ao
// contrário do modelo persistido, seus trechos já possuem informações legíveis.
type ReservationResponse struct {
	ID        uuid.UUID                  `json:"id"`
	Status    enum.ReservationStatus     `json:"status"`
	CreatedAt time.Time                  `json:"created_at"`
	Segments  []ReservationStageResponse `json:"segments"`
}

type ReservationStageResponse struct {
	RideID      uuid.UUID `json:"ride_id"`
	SegmentID   uuid.UUID `json:"segment_id"`
	Origin      enum.City `json:"origin"`
	Destination enum.City `json:"destination"`
	DepartureAt time.Time `json:"departure_at"`
	ArrivalAt   time.Time `json:"arrival_at"`
	PriceCents  int       `json:"price_cents"`
}
