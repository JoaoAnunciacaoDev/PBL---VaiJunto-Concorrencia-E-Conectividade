package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/utils"
	"github.com/google/uuid"
)

const searchDateLayout = "02/01/2006"

func passengerMenu(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) enum.MenuResult {
	for {
		fmt.Fprintln(output, "\n=== Terminal do Passageiro ===")
		fmt.Fprintln(output, "1 - Buscar itinerários")
		fmt.Fprintln(output, "2 - Minhas reservas")
		fmt.Fprintln(output, "3 - Cancelar reserva")
		fmt.Fprintln(output, "0 - Voltar")

		choice, err := readLine(input, output, "Opção: ")
		if err != nil {
			return enum.MenuBack
		}

		switch choice {
		case "1":
			utils.ClearTerminal()
			if !searchItineraries(tcpClient, input, output) {
				return enum.MenuDisconnected
			}

		case "2":
			utils.ClearTerminal()
			if !listMyReservations(tcpClient, output) {
				return enum.MenuDisconnected
			}

		case "3":
			utils.ClearTerminal()
			if !cancelReservation(tcpClient, input, output) {
				return enum.MenuDisconnected
			}

		case "0":
			return enum.MenuBack

		default:
			fmt.Fprintln(output, "Opção inválida.")
		}
	}
}

// searchItineraries solicita ao usuário informações sobre a viagem desejada, 
// envia uma solicitação de busca ao servidor e exibe os itinerários encontrados.
// Retorna um booleano indicando se a conexão com o servidor foi mantida.
func searchItineraries(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) bool {
	origin, ok := chooseCity(input, output, "Origem")
	if !ok {
		return true
	}

	destination, ok := chooseCity(input, output, "Destino")
	if !ok {
		return true
	}

	dateText, err := readLine(input, output, "Data da viagem (dd/mm/aaaa): ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler a data.")
		return true
	}

	date, err := time.ParseInLocation(searchDateLayout, dateText, time.Local)
	if err != nil {
		fmt.Fprintln(output, "Data inválida.")
		return true
	}

	payload, err := json.Marshal(protocol.SearchItinerariesRequest{Origin: origin, Destination: destination, Date: date})
	if err != nil {
		fmt.Fprintln(output, "Não foi possível preparar a busca.")
		return true
	}

	response, err := tcpClient.Send(protocol.Request{Action: protocol.ActionSearchItineraries, Payload: payload})
	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return false
	}

	if response.Success != "success" {
		fmt.Fprintln(output, response.Message)
		return true
	}

	var itineraries []protocol.ItineraryResponse
	if err := json.Unmarshal(response.Payload, &itineraries); err != nil {
		fmt.Fprintln(output, "O servidor retornou uma resposta inválida.")
		return true
	}

	if len(itineraries) == 0 {
		fmt.Fprintln(output, "Nenhum itinerário disponível para a busca.")
		return true
	}

	fmt.Fprintln(output, response.Message)

	for index, itinerary := range itineraries {
		printItinerary(output, index+1, itinerary)
	}

	choiceText, err := readLine(input, output, "Número do itinerário para confirmar ou 0 para voltar: ")
	if err != nil {
		return true
	}

	choice, err := strconv.Atoi(choiceText)
	if err != nil || choice < 0 || choice > len(itineraries) {
		fmt.Fprintln(output, "Opção inválida.")
		return true
	}

	if choice == 0 {
		return true
	}

	return confirmItinerary(tcpClient, input, output, itineraries[choice-1])
}

// confirmItinerary solicita ao usuário a confirmação da reserva do itinerário selecionado,
// envia uma solicitação de confirmação ao servidor e exibe a mensagem de resposta.
// Retorna um booleano indicando se a conexão com o servidor foi mantida.
func confirmItinerary(tcpClient *TCPClient, input *bufio.Reader, output io.Writer, 
	itinerary protocol.ItineraryResponse) bool {
	confirmation, err := readLine(input, output, "Confirmar reserva? (s/N): ")
	if err != nil || strings.ToLower(confirmation) != "s" {
		fmt.Fprintln(output, "Reserva cancelada.")
		return true
	}

	segments := make([]models.ReservedSegment, 0, len(itinerary.Segments))
	for _, segment := range itinerary.Segments {
		segments = append(segments, models.ReservedSegment{RideID: segment.RideID, SegmentID: segment.SegmentID})
	}

	payload, err := json.Marshal(protocol.ConfirmReservationRequest{Segments: segments})
	if err != nil {
		return true
	}

	response, err := tcpClient.Send(protocol.Request{Action: protocol.ActionConfirmReservation, Payload: payload})
	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return false
	}

	if response.Success != "success" {
		fmt.Fprintln(output, response.Message)
		return true
	}

	var reservation models.Reservation
	if err := json.Unmarshal(response.Payload, &reservation); err != nil {
		fmt.Fprintln(output, "O servidor retornou uma resposta inválida.")
		return true
	}

	fmt.Fprintf(output, "%s ID da reserva: %s\n", response.Message, reservation.ID)
	return true
}

// listMyReservations solicita ao servidor a lista de reservas do usuário autenticado e as exibe no terminal.
// Retorna um booleano indicando se a conexão com o servidor foi mantida.
func listMyReservations(tcpClient *TCPClient, output io.Writer) bool {
	response, err := tcpClient.Send(protocol.Request{Action: protocol.ActionListMyReservations})
	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return false
	}

	if response.Success != "success" {
		fmt.Fprintln(output, response.Message)
		return true
	}

	var reservations []models.Reservation
	if err := json.Unmarshal(response.Payload, &reservations); err != nil {
		fmt.Fprintln(output, "O servidor retornou uma resposta inválida.")
		return true
	}

	if len(reservations) == 0 {
		fmt.Fprintln(output, "Você ainda não possui reservas.")
		return true
	}

	for _, reservation := range reservations {
		fmt.Fprintf(output, "\nID: %s\nStatus: %s\nTrechos: %d\n", reservation.ID, reservation.Status, len(reservation.Segments))
	}

	return true
}

// cancelReservation envia uma solicitação de cancelamento de reserva ao servidor.
// Retorna um booleano indicando se a conexão com o servidor foi mantida.
func cancelReservation(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) bool {
	reservationIDText, err := readLine(input, output, "ID da reserva: ")
	if err != nil {
		return true
	}

	reservationID, err := uuid.Parse(reservationIDText)
	if err != nil {
		fmt.Fprintln(output, "ID da reserva inválido.")
		return true
	}

	payload, err := json.Marshal(protocol.CancelReservationRequest{ReservationID: reservationID})
	if err != nil {
		return true
	}

	response, err := tcpClient.Send(protocol.Request{Action: protocol.ActionCancelReservation, Payload: payload})
	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return false
	}

	fmt.Fprintln(output, response.Message)
	
	return true
}

func printItinerary(output io.Writer, index int, itinerary protocol.ItineraryResponse) {
	fmt.Fprintf(output, "\n=== Itinerário %d ===\nPartida: %s\nChegada: %s\n", index, itinerary.DepartureAt.Format(rideDateTimeLayout), itinerary.ArrivalAt.Format(rideDateTimeLayout))
	for _, segment := range itinerary.Segments {
		fmt.Fprintf(output, "%s → %s | %s até %s | %s\n", segment.Origin, segment.Destination, segment.DepartureAt.Format(rideDateTimeLayout), segment.ArrivalAt.Format(rideDateTimeLayout), formatCents(segment.PriceCents))
	}
	fmt.Fprintf(output, "Preço total: %s\n", formatCents(itinerary.TotalPriceCents))
}
