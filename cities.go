package enums

type City int

const (
	FeiraDeSantana City = iota
	Alagoinhas
	Salvador
	LauroDeFreitas
	Camacari
	VitoriaDaConquista
	Jequie
	Lencois
)

var cities = map[City] string {
	FeiraDeSantana: "Feira de Santana",
	Alagoinhas: "Alagoinhas",
	Salvador: "Salvador",
	LauroDeFreitas: "Lauro de Freitas",
	Camacari: "Camaçari"
	VitoriaDaConquista: "Vitória da Conquista"
	Jequie: "Jequié"
	Lencois: "Lençóis",
}