package server

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/google/uuid"
)

const minimumConnectionTime = 15 * time.Minute

type itineraryEdge struct {
	rideID uuid.UUID
	stage  models.Stage
}

func (s *Server) handleSearchItineraries(payload json.RawMessage, session *Session) protocol.Response {
	if response, ok := s.passengerForSession(session); !ok {
		return response
	}

	var dto protocol.SearchItinerariesRequest
	if err := json.Unmarshal(payload, &dto); err != nil {
		return protocol.Response{Success: "error", Message: "Requisição inválida: " + err.Error()}
	}
	if !dto.Origin.IsValid() || !dto.Destination.IsValid() || dto.Origin == dto.Destination || dto.Date.IsZero() {
		return protocol.Response{Success: "error", Message: "Origem, destino e data válidos são obrigatórios."}
	}

	rides, err := s.repository.GetActiveRidesByDate(dto.Date)
	if err != nil {
		return protocol.Response{Success: "error", Message: "Não foi possível buscar caronas."}
	}

	itineraries := searchItineraries(rides, dto.Origin, dto.Destination)
	return rideResponse(itineraries, "Itinerários obtidos com sucesso.")
}

func (s *Server) passengerForSession(session *Session) (protocol.Response, bool) {
	if !session.IsAuthenticated() {
		return protocol.Response{Success: "error", Message: "Autenticação necessária."}, false
	}

	user, err := s.repository.GetUserByID(session.UserID)
	if err != nil {
		session.Clear()
		return protocol.Response{Success: "error", Message: "Sessão inválida. Faça login novamente."}, false
	}
	if user.Role != models.RolePassenger {
		return protocol.Response{Success: "error", Message: "Apenas passageiros podem buscar itinerários."}, false
	}

	return protocol.Response{}, true
}

func searchItineraries(rides []*models.Ride, origin, destination enum.City) []protocol.ItineraryResponse {
	edgesByOrigin := make(map[enum.City][]itineraryEdge)
	for _, ride := range rides {
		for _, stage := range ride.Segments {
			if stage.AvailableSeats > 0 {
				edgesByOrigin[stage.Origin] = append(edgesByOrigin[stage.Origin], itineraryEdge{rideID: ride.ID, stage: stage})
			}
		}
	}

	for city := range edgesByOrigin {
		sort.Slice(edgesByOrigin[city], func(i, j int) bool {
			return edgesByOrigin[city][i].stage.DepartureAt.Before(edgesByOrigin[city][j].stage.DepartureAt)
		})
	}

	itineraries := make([]protocol.ItineraryResponse, 0)
	seen := make(map[string]struct{})
	searchItineraryPaths(edgesByOrigin, origin, destination, nil, map[enum.City]bool{origin: true}, &itineraries, seen)

	sort.Slice(itineraries, func(i, j int) bool {
		if itineraries[i].TotalPriceCents != itineraries[j].TotalPriceCents {
			return itineraries[i].TotalPriceCents < itineraries[j].TotalPriceCents
		}
		return itineraries[i].DepartureAt.Before(itineraries[j].DepartureAt)
	})

	return itineraries
}

func searchItineraryPaths(edgesByOrigin map[enum.City][]itineraryEdge, currentCity, destination enum.City, path []itineraryEdge, visitedCities map[enum.City]bool, itineraries *[]protocol.ItineraryResponse, seen map[string]struct{}) {
	for _, edge := range edgesByOrigin[currentCity] {
		if visitedCities[edge.stage.Destination] || !canConnect(path, edge) {
			continue
		}

		nextPath := append(append([]itineraryEdge(nil), path...), edge)
		if edge.stage.Destination == destination {
			itinerary := itineraryFromPath(nextPath)
			key := itineraryKey(itinerary)
			if _, exists := seen[key]; !exists {
				seen[key] = struct{}{}
				*itineraries = append(*itineraries, itinerary)
			}
			continue
		}

		visitedCities[edge.stage.Destination] = true
		searchItineraryPaths(edgesByOrigin, edge.stage.Destination, destination, nextPath, visitedCities, itineraries, seen)
		delete(visitedCities, edge.stage.Destination)
	}
}

func canConnect(path []itineraryEdge, next itineraryEdge) bool {
	if len(path) == 0 {
		return true
	}

	previous := path[len(path)-1]
	minimumDeparture := previous.stage.ArrivalAt
	if previous.rideID != next.rideID {
		minimumDeparture = minimumDeparture.Add(minimumConnectionTime)
	}

	return !next.stage.DepartureAt.Before(minimumDeparture)
}

func itineraryFromPath(path []itineraryEdge) protocol.ItineraryResponse {
	segments := make([]protocol.ItinerarySegmentResponse, 0, len(path))
	totalPriceCents := 0
	for _, edge := range path {
		segments = append(segments, protocol.ItinerarySegmentResponse{
			RideID:         edge.rideID,
			SegmentID:      edge.stage.ID,
			Origin:         edge.stage.Origin,
			Destination:    edge.stage.Destination,
			DepartureAt:    edge.stage.DepartureAt,
			ArrivalAt:      edge.stage.ArrivalAt,
			PriceCents:     edge.stage.PriceCents,
			AvailableSeats: edge.stage.AvailableSeats,
		})
		totalPriceCents += edge.stage.PriceCents
	}

	return protocol.ItineraryResponse{
		Segments:        segments,
		DepartureAt:     segments[0].DepartureAt,
		ArrivalAt:       segments[len(segments)-1].ArrivalAt,
		TotalPriceCents: totalPriceCents,
	}
}

func itineraryKey(itinerary protocol.ItineraryResponse) string {
	segmentIDs := make([]string, 0, len(itinerary.Segments))
	for _, segment := range itinerary.Segments {
		segmentIDs = append(segmentIDs, segment.RideID.String()+":"+segment.SegmentID.String())
	}
	return strings.Join(segmentIDs, "|")
}
