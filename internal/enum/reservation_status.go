package enum

type ReservationStatus int

const (
	ReservationStatusUnknown ReservationStatus = iota
	Pendente
	Cancelada
	Confirmada
)

var reservationStatusNames = map[ReservationStatus]string{
	Pendente:   "Pendente",
	Cancelada:  "Cancelada",
	Confirmada: "Confirmada",
}

func (s ReservationStatus) String() string {
	return reservationStatusNames[s]
}

func (s ReservationStatus) IsValid() bool {
	_, exists := reservationStatusNames[s]
	return exists
}
