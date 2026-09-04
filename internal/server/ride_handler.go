package server

import (
	"encoding/json"
	"fmt"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/google/uuid"
)

func (s *Server) handleCreateRide(payload json.RawMessage, session *Session) protocol.Response {
	driver, response, ok := s.driverForSession(session, true)
	if !ok {
		return response
	}

	var dto protocol.CreateRideRequest
	if err := json.Unmarshal(payload, &dto); err != nil {
		return protocol.Response{Success: "error", Message: "Requisição inválida: " + err.Error()}
	}

	if dto.DepartureAt.IsZero() || len(dto.Segments) == 0 {
		return protocol.Response{Success: "error", Message: "Data de partida e ao menos um trecho são obrigatórios."}
	}

	segments := make([]models.Stage, 0, len(dto.Segments))
	for index, requestedSegment := range dto.Segments {
		segment := models.Stage{
			ID:             uuid.New(),
			Origin:         requestedSegment.Origin,
			Destination:    requestedSegment.Destination,
			DepartureAt:    requestedSegment.DepartureAt,
			ArrivalAt:      requestedSegment.ArrivalAt,
			PriceCents:     requestedSegment.PriceCents,
			AvailableSeats: requestedSegment.AvailableSeats,
		}

		if !segment.IsValid() || segment.AvailableSeats == 0 {
			return protocol.Response{Success: "error", Message: fmt.Sprintf("Trecho %d é inválido.", index+1)}
		}
		if segment.AvailableSeats > driver.Vehicle.SeatCapacity {
			return protocol.Response{Success: "error", Message: fmt.Sprintf("Trecho %d excede a capacidade do veículo.", index+1)}
		}

		segments = append(segments, segment)
	}

	ride := &models.Ride{
		ID:          uuid.New(),
		DriverID:    driver.ID,
		DepartureAt: dto.DepartureAt,
		Segments:    segments,
	}
	if !ride.IsValid() {
		return protocol.Response{Success: "error", Message: "A rota da carona deve possuir trechos contínuos, em ordem de horário e válidos."}
	}

	if err := s.repository.SaveRide(ride); err != nil {
		return protocol.Response{Success: "error", Message: "Não foi possível publicar a carona."}
	}

	return rideResponse(ride, "Carona publicada com sucesso.")
}

func (s *Server) handleListMyRides(session *Session) protocol.Response {
	driver, response, ok := s.driverForSession(session, false)
	if !ok {
		return response
	}

	rides, err := s.repository.GetRidesByDriverID(driver.ID)
	if err != nil {
		return protocol.Response{Success: "error", Message: "Não foi possível consultar as caronas."}
	}

	return rideResponse(rides, "Caronas obtidas com sucesso.")
}

func (s *Server) handleCancelRide(payload json.RawMessage, session *Session) protocol.Response {
	driver, response, ok := s.driverForSession(session, false)
	if !ok {
		return response
	}

	var dto protocol.CancelRideRequest
	if err := json.Unmarshal(payload, &dto); err != nil || dto.RideID == uuid.Nil {
		return protocol.Response{Success: "error", Message: "Identificador da carona inválido."}
	}

	if err := s.repository.CancelRide(dto.RideID, driver.ID); err != nil {
		return protocol.Response{Success: "error", Message: err.Error()}
	}

	return protocol.Response{Success: "success", Message: "Carona cancelada com sucesso."}
}

func rideResponse(data any, message string) protocol.Response {
	payload, err := json.Marshal(data)
	if err != nil {
		return protocol.Response{Success: "error", Message: "Não foi possível processar a resposta."}
	}

	return protocol.Response{Success: "success", Message: message, Payload: payload}
}
