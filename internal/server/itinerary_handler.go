package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/google/uuid"
)

type itineraryEdge struct {
	companyID string
	rideID    uuid.UUID
	stage     models.Stage
}

type RideSource struct {
	CompanyID string
	Ride      *models.Ride
}

// handleSearchItineraries lida com a solicitação de busca de itinerários.
// Ele verifica se o usuário está autenticado, valida os parâmetros da solicitação,
// busca caronas locais e consulta os servidores de outras empresas do consórcio via API REST,
// compondo itinerários que conectem a origem e o destino através de qualquer empresa do consórcio.
func (s *Server) handleSearchItineraries(payload json.RawMessage, session *Session) protocol.Response {
	if response, ok := s.passengerForSession(session); !ok {
		return response
	}

	var dto protocol.SearchItinerariesRequest
	if err := protocol.UnmarshalStrict(payload, &dto); err != nil {
		return protocol.Response{Success: "error", Message: "Requisição inválida: " + err.Error()}
	}
	if !dto.Origin.IsValid() || !dto.Destination.IsValid() || dto.Origin == dto.Destination || dto.Date.IsZero() {
		return protocol.Response{Success: "error", Message: "Origem, destino e data válidos são obrigatórios."}
	}

	var sources []RideSource

	// 1. Caronas locais
	localRides, err := s.repository.GetActiveRidesByDate(dto.Date)
	if err == nil {
		for _, ride := range localRides {
			sources = append(sources, RideSource{
				CompanyID: s.companyID,
				Ride:      ride,
			})
		}
	}

	// 2. Consulta paralela/tolerante a falhas aos servidores parceiros do consórcio
	if s.peers != nil {
		for _, peer := range s.peers.All() {
			remoteRides, err := s.fetchRemoteRides(peer.InternalURL, dto.Date)
			if err != nil {
				// Queda ou lentidão de um servidor parceiro é tolerada: os demais continuam operando
				continue
			}
			for _, ride := range remoteRides {
				sources = append(sources, RideSource{
					CompanyID: peer.CompanyID,
					Ride:      ride,
				})
			}
		}
	}

	itineraries := searchItinerariesFromSources(sources, dto.Origin, dto.Destination)
	return rideResponse(itineraries, "Itinerários obtidos com sucesso.")
}

func (s *Server) fetchRemoteRides(baseURL string, date time.Time) ([]*models.Ride, error) {
	url := fmt.Sprintf("%s/internal/v1/rides?date=%s", baseURL, date.Format("2006-01-02"))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("peer retornou status %d", resp.StatusCode)
	}

	var rides []*models.Ride
	if err := json.NewDecoder(resp.Body).Decode(&rides); err != nil {
		return nil, err
	}

	return rides, nil
}

func (s *Server) passengerForSession(session *Session) (protocol.Response, bool) {
	if !session.IsAuthenticated() {
		return protocol.Response{Success: "error", Message: "Autenticação necessária."}, false
	}

	_, err := s.repository.GetUserByID(session.UserID)
	if err != nil {
		session.Clear()
		return protocol.Response{Success: "error", Message: "Sessão inválida. Faça login novamente."}, false
	}

	return protocol.Response{}, true
}

// searchItineraries é mantido para compatibilidade com testes unitários locais existentes.
func searchItineraries(rides []*models.Ride, origin, destination enum.City) []protocol.ItineraryResponse {
	sources := make([]RideSource, 0, len(rides))
	for _, ride := range rides {
		sources = append(sources, RideSource{
			CompanyID: "empresa-a",
			Ride:      ride,
		})
	}
	return searchItinerariesFromSources(sources, origin, destination)
}

func searchItinerariesFromSources(sources []RideSource, origin, destination enum.City) []protocol.ItineraryResponse {
	edgesByOrigin := make(map[enum.City][]itineraryEdge)
	for _, src := range sources {
		for _, stage := range src.Ride.Segments {
			if stage.AvailableSeats > 0 {
				edgesByOrigin[stage.Origin] = append(edgesByOrigin[stage.Origin], itineraryEdge{
					companyID: src.CompanyID,
					rideID:    src.Ride.ID,
					stage:     stage,
				})
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
	return !next.stage.DepartureAt.Before(previous.stage.ArrivalAt)
}

func itineraryFromPath(path []itineraryEdge) protocol.ItineraryResponse {
	segments := make([]protocol.ItinerarySegmentResponse, 0, len(path))
	totalPriceCents := 0
	for _, edge := range path {
		segments = append(segments, protocol.ItinerarySegmentResponse{
			RideID:         edge.rideID,
			SegmentID:      edge.stage.ID,
			CompanyID:      edge.companyID,
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
