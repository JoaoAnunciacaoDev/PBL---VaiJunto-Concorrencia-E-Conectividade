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
	CreatedAt   time.Time              `json:"created_at"`
}
