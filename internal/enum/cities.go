package enum

type City int

const (
	CityUnknown City = iota
	FeiraDeSantana
	Alagoinhas
	Salvador
	LauroDeFreitas
	Camacari
	VitoriaDaConquista
	Jequie
	Lencois
)

var cityNames = map[City]string{
	FeiraDeSantana:     "Feira de Santana",
	Alagoinhas:         "Alagoinhas",
	Salvador:           "Salvador",
	LauroDeFreitas:     "Lauro de Freitas",
	Camacari:           "Camaçari",
	VitoriaDaConquista: "Vitória da Conquista",
	Jequie:             "Jequié",
	Lencois:            "Lençóis",
}

var allCities = []City{
	FeiraDeSantana,
	Alagoinhas,
	Salvador,
	LauroDeFreitas,
	Camacari,
	VitoriaDaConquista,
	Jequie,
	Lencois,
}

func Cities() []City {
	return append([]City(nil), allCities...)
}

func (c City) String() string {
	return cityNames[c]
}

func (c City) IsValid() bool {
	_, exists := cityNames[c]
	return exists
}
