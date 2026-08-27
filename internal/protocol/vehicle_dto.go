package protocol

type CreateVehicleRequest struct {
	Plate        string `json:"plate"`
	Model        string `json:"model"`
	Color        string `json:"color"`
	SeatCapacity int    `json:"seat_capacity"`
}
