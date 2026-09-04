package server

import (
	"encoding/json"

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

	return reservationResponse(reservations, "Reservas obtidas com sucesso.")
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
