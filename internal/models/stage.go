package models

import "github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"

type Stage struct {
	Origin         enum.City `json:"origin"`
	Destination    enum.City `json:"destination"`
	PriceCents     int       `json:"price_cents"`
	AvailableSeats int       `json:"available_seats"`
}

func (s Stage) IsValid() bool {
	return s.Origin.IsValid() &&
		s.Destination.IsValid() &&
		s.Origin != s.Destination &&
		s.PriceCents >= 0 &&
		s.AvailableSeats >= 0
}
