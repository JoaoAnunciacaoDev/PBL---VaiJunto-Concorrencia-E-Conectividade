package server

import (
	"encoding/json"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/google/uuid"
)

func (s *Server) handleConfirmReservation(payload json.RawMessage, session *Session) protocol.Response {
	if response, ok := s.passengerForSession(session); !ok {
		return response
	}

	var dto protocol.ConfirmReservationRequest
	if err := protocol.UnmarshalStrict(payload, &dto); err != nil {
		return protocol.Response{Success: "error", Message: "Requisição inválida: " + err.Error()}
	}

	if len(dto.Segments) == 0 {
		return protocol.Response{Success: "error", Message: "Nenhum trecho informado para reserva."}
	}

	// Normaliza o CompanyID para segmentos que porventura não vieram preenchidos
	for i := range dto.Segments {
		if dto.Segments[i].CompanyID == "" {
			dto.Segments[i].CompanyID = s.companyID
		}
	}

	// Se o coordenador 2PC estiver habilitado, executa o Two-Phase Commit distribuído
	if s.coordinator != nil {
		if err := s.coordinator.ConfirmItinerary(session.UserID, dto.Segments); err != nil {
			return protocol.Response{Success: "error", Message: err.Error()}
		}

		reservation, err := s.repository.RegisterConfirmedReservation(session.UserID, dto.Segments)
		if err != nil {
			return protocol.Response{Success: "error", Message: "Erro ao registrar reserva: " + err.Error()}
		}

		return reservationResponse(reservation, "Reserva confirmada com sucesso.")
	}

	// Fallback para modo local puro
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
		var stage *models.Stage

		// Se a carona é local, busca do repositório local
		ride, err := s.repository.GetRideByID(reservedSegment.RideID)
		if err == nil {
			for index := range ride.Segments {
				if ride.Segments[index].ID == reservedSegment.SegmentID {
					stage = &ride.Segments[index]
					break
				}
			}
		}

		// Se não encontrou localmente e pertence a um parceiro, busca do peer
		if stage == nil && s.peers != nil && reservedSegment.CompanyID != "" && reservedSegment.CompanyID != s.companyID {
			if peer, exists := s.peers.Get(reservedSegment.CompanyID); exists {
				remoteRides, rErr := s.fetchRemoteRides(peer.InternalURL, reservation.CreatedAt)
				if rErr == nil {
					for _, r := range remoteRides {
						if r.ID == reservedSegment.RideID {
							for index := range r.Segments {
								if r.Segments[index].ID == reservedSegment.SegmentID {
									stage = &r.Segments[index]
									break
								}
							}
						}
					}
				}
			}
		}

		var origin, dest enum.City
		var depAt, arrAt time.Time
		var price int
		if stage != nil {
			origin = stage.Origin
			dest = stage.Destination
			depAt = stage.DepartureAt
			arrAt = stage.ArrivalAt
			price = stage.PriceCents
		}

		segments = append(segments, protocol.ReservationStageResponse{
			RideID:      reservedSegment.RideID,
			SegmentID:   reservedSegment.SegmentID,
			Origin:      origin,
			Destination: dest,
			DepartureAt: depAt,
			ArrivalAt:   arrAt,
			PriceCents:  price,
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
	if err := protocol.UnmarshalStrict(payload, &dto); err != nil || dto.ReservationID == uuid.Nil {
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
