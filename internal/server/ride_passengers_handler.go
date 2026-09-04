package server

import (
	"encoding/json"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/google/uuid"
)

func (s *Server) handleGetRidePassengers(payload json.RawMessage, session *Session) protocol.Response {
	driver, response, ok := s.driverForSession(session, false)
	if !ok {
		return response
	}

	var dto protocol.GetRidePassengersRequest
	if err := json.Unmarshal(payload, &dto); err != nil || dto.RideID == uuid.Nil {
		return protocol.Response{Success: "error", Message: "Identificador da carona inválido."}
	}

	ride, err := s.repository.GetRideByID(dto.RideID)
	if err != nil {
		return protocol.Response{Success: "error", Message: err.Error()}
	}
	if ride.DriverID != driver.ID {
		return protocol.Response{Success: "error", Message: "Motorista não pode consultar passageiros desta carona."}
	}

	passengerIDsBySegment, err := s.repository.GetConfirmedPassengerIDsByRideID(ride.ID)
	if err != nil {
		return protocol.Response{Success: "error", Message: "Não foi possível consultar os passageiros."}
	}

	segments := make([]protocol.RideSegmentPassengersResponse, 0, len(ride.Segments))
	for _, stage := range ride.Segments {
		passengers := make([]protocol.UserResponse, 0, len(passengerIDsBySegment[stage.ID]))
		for _, passengerID := range passengerIDsBySegment[stage.ID] {
			passenger, err := s.repository.GetUserByID(passengerID)
			if err != nil {
				return protocol.Response{Success: "error", Message: "Não foi possível consultar os passageiros."}
			}
			passengers = append(passengers, protocol.UserResponse{
				ID: passenger.ID, Name: passenger.Name, Email: passenger.Email, Role: passenger.Role,
			})
		}

		segments = append(segments, protocol.RideSegmentPassengersResponse{
			SegmentID: stage.ID, Origin: stage.Origin, Destination: stage.Destination,
			DepartureAt: stage.DepartureAt, ArrivalAt: stage.ArrivalAt, Passengers: passengers,
		})
	}

	return rideResponse(protocol.RidePassengersResponse{RideID: ride.ID, Segments: segments}, "Passageiros obtidos com sucesso.")
}
