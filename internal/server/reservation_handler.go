package server

import (
	"encoding/json"
	"fmt"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/google/uuid"
)

func (s *Server) handleConfirmReservation(payload json.RawMessage, session *Session) protocol.Response {
	if response, ok := s.passengerForSession(session); !ok {
		return response
	}

	var dto protocol.ConfirmReservationRequest
	if err := json.Unmarshal(payload, &dto); err != nil {
		return protocol.Response{Success: "error", Message: "Requisição inválida: " + err.Error()}
	}

	reservation, err := s.repository.ConfirmReservation(session.UserID, dto.Segments)
	if err != nil {
		return protocol.Response{Success: "error", Message: err.Error()}
	}

	return reservationResponse(reservation, "Reserva confirmada com sucesso.")
}

func (s *Server) handleListMyReservations(session *Session) protocol.Response {
	if response, ok := s.passengerForSession(session); !ok {
		return response
	}

	reservations, err := s.repository.GetReservationsByPassengerID(session.UserID)
	if err != nil {
		return protocol.Response{Success: "error", Message: "Não foi possível consultar as reservas."}
	}

	responses := make([]protocol.ReservationResponse, 0, len(reservations))
	for _, reservation := range reservations {
		response, err := s.reservationResponse(*reservation)
		if err != nil {
			return protocol.Response{Success: "error", Message: "Não foi possível consultar os trechos das reservas."}
		}
		responses = append(responses, response)
	}

	return reservationResponse(responses, "Reservas obtidas com sucesso.")
}

func (s *Server) reservationResponse(reservation models.Reservation) (protocol.ReservationResponse, error) {
	segments := make([]protocol.ReservationStageResponse, 0, len(reservation.Segments))
	for _, reservedSegment := range reservation.Segments {
		ride, err := s.repository.GetRideByID(reservedSegment.RideID)
		if err != nil {
			return protocol.ReservationResponse{}, err
		}

		var stage *models.Stage
		for index := range ride.Segments {
			if ride.Segments[index].ID == reservedSegment.SegmentID {
				stage = &ride.Segments[index]
				break
			}
		}
		if stage == nil {
			return protocol.ReservationResponse{}, fmt.Errorf("trecho da reserva não encontrado")
		}

		segments = append(segments, protocol.ReservationStageResponse{
			RideID:      reservedSegment.RideID,
			SegmentID:   reservedSegment.SegmentID,
			Origin:      stage.Origin,
			Destination: stage.Destination,
			DepartureAt: stage.DepartureAt,
			ArrivalAt:   stage.ArrivalAt,
			PriceCents:  stage.PriceCents,
		})
	}

	return protocol.ReservationResponse{
		ID:        reservation.ID,
		Status:    reservation.Status,
		CreatedAt: reservation.CreatedAt,
		Segments:  segments,
	}, nil
}

func (s *Server) handleCancelReservation(payload json.RawMessage, session *Session) protocol.Response {
	if response, ok := s.passengerForSession(session); !ok {
		return response
	}

	var dto protocol.CancelReservationRequest
	if err := json.Unmarshal(payload, &dto); err != nil || dto.ReservationID == uuid.Nil {
		return protocol.Response{Success: "error", Message: "Identificador da reserva inválido."}
	}

	if err := s.repository.CancelReservation(dto.ReservationID, session.UserID); err != nil {
		return protocol.Response{Success: "error", Message: err.Error()}
	}

	return protocol.Response{Success: "success", Message: "Reserva cancelada com sucesso."}
}

func reservationResponse(data any, message string) protocol.Response {
	payload, err := json.Marshal(data)
	if err != nil {
		return protocol.Response{Success: "error", Message: "Não foi possível processar a resposta."}
	}

	return protocol.Response{Success: "success", Message: message, Payload: payload}
}
