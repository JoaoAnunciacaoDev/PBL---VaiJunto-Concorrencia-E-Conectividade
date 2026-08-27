package models

import (
	"errors"
)

type Driver struct {
	User
	Vehicle *Vehicle `json:"vehicle,omitempty"`
}

func (d *Driver) RegisterVehicle(vehicle Vehicle) {
	d.Vehicle = &vehicle
}

func (d *Driver) UpdateVehicle(plate, model, color string, seatCapacity int) error {
	if d.Vehicle == nil {
		return errors.New("no vehicle registered")
	}

	d.Vehicle.Plate = plate
	d.Vehicle.Model = model
	d.Vehicle.Color = color
	d.Vehicle.SeatCapacity = seatCapacity

	return nil
}

func (d *Driver) RemoveVehicle() {
	d.Vehicle = nil
}

func (d *Driver) HasVehicle() bool {
	return d.Vehicle != nil
}
