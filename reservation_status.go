package enums

type ReservationStatus int

const (
	Pendente ReservationStatus = iota
	Cancelada
	Confirmada
)

var status = map[ReservationStatus] string {
	Pendente: "Pendente",
	Cancelada: "Cancelada",
	Confirmada: "Confirmada",
}