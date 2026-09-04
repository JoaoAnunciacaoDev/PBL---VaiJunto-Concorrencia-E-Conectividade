package models

import (
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/google/uuid"
)

type Stage struct {
	ID             uuid.UUID `json:"id"`
	Origin         enum.City `json:"origin"`
	Destination    enum.City `json:"destination"`
	DepartureAt    time.Time `json:"departure_at"`
	ArrivalAt      time.Time `json:"arrival_at"`
	PriceCents     int       `json:"price_cents"`
	AvailableSeats int       `json:"available_seats"`
}

func (s Stage) IsValid() bool {
	return s.ID != uuid.Nil &&
		s.Origin.IsValid() &&
		s.Destination.IsValid() &&
		s.Origin != s.Destination &&
		!s.DepartureAt.IsZero() &&
		s.ArrivalAt.After(s.DepartureAt) &&
		s.PriceCents >= 0 &&
		s.AvailableSeats >= 0
}
