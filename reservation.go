package models

import (
	"time"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enums"
)

type Reservation struct {
	reservation_date time.TIME				`json:"date"`
	reservation_status ReservationStatus	`json:"reservation_status"`
}