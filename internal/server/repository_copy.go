package server

import "github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"

func cloneUser(user *models.User) *models.User {
	if user == nil {
		return nil
	}
	copy := *user
	return &copy
}

func cloneDriver(driver *models.Driver) *models.Driver {
	if driver == nil {
		return nil
	}
	copy := *driver
	if driver.Vehicle != nil {
		vehicleCopy := *driver.Vehicle
		copy.Vehicle = &vehicleCopy
	}
	return &copy
}

func cloneRide(ride *models.Ride) *models.Ride {
	if ride == nil {
		return nil
	}
	copy := *ride
	copy.Segments = append([]models.Stage(nil), ride.Segments...)
	return &copy
}

func cloneReservation(reservation *models.Reservation) *models.Reservation {
	if reservation == nil {
		return nil
	}
	copy := *reservation
	copy.Segments = append([]models.ReservedSegment(nil), reservation.Segments...)
	return &copy
}
