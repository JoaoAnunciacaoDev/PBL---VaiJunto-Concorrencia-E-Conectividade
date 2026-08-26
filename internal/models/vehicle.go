package models

type Vehicle struct {
	Plate string `json:"plate"`
	Model string `json:"model"`
	Color string `json:"color"`
	AvailableSeats int `json:"available_seats"`	
}