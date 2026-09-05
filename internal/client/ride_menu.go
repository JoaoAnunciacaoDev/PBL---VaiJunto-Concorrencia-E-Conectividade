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
)

const rideDateTimeLayout = "02/01/2006 15:04"

func rideMenu(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) enum.MenuResult {
	for {
		fmt.Fprintln(output, "\n=== Caronas ===")
		fmt.Fprintln(output, "1 - Publicar carona")
		fmt.Fprintln(output, "2 - Minhas caronas")
		fmt.Fprintln(output, "3 - Cancelar carona")
		fmt.Fprintln(output, "4 - Passageiros por trecho")
		fmt.Fprintln(output, "0 - Voltar")

		choice, err := readLine(input, output, "Opção: ")
		if err != nil {
			return enum.MenuBack
		}

		switch choice {
		case "1":
			utils.ClearTerminal()
			if !createRide(tcpClient, input, output) {
				return enum.MenuDisconnected
			}
			if !waitForEnter(input, output) {
				return enum.MenuBack
			}
			utils.ClearTerminal()

		case "2":
			utils.ClearTerminal()
			if !listMyRides(tcpClient, output) {
				return enum.MenuDisconnected
			}
			if !waitForEnter(input, output) {
				return enum.MenuBack
			}
			utils.ClearTerminal()

		case "3":
			utils.ClearTerminal()
			if !cancelRide(tcpClient, input, output) {
				return enum.MenuDisconnected
			}
			if !waitForEnter(input, output) {
				return enum.MenuBack
			}
			utils.ClearTerminal()

		case "4":
			utils.ClearTerminal()
			if !listRidePassengers(tcpClient, input, output) {
				return enum.MenuDisconnected
			}
			if !waitForEnter(input, output) {
				return enum.MenuBack
			}
			utils.ClearTerminal()

		case "0":
			utils.ClearTerminal()
			return enum.MenuBack
		default:
			fmt.Fprintln(output, "Opção inválida.")
		}
	}
}

func createRide(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) bool {
	vehicleAvailable, connected := ensureVehicle(tcpClient, output)
	if !connected {
		return false
	}
	if !vehicleAvailable {
		return true
	}

	fmt.Fprintln(output, "=== Publicar carona ===")
	fmt.Fprintln(output, "Informe a origem inicial. Cada próximo trecho começa onde o anterior terminou.")

	departureAt, ok := readDateTime(input, output, "Data e horário de partida (dd/mm/aaaa hh:mm): ")
	if !ok {
		return true
	}
	segmentCount, ok := readPositiveInt(input, output, "Quantidade de trechos: ")
	if !ok {
		return true
	}
	currentOrigin, ok := chooseCity(input, output, "Origem inicial")
	if !ok {
		return true
	}

	segments := make([]protocol.CreateStageRequest, 0, segmentCount)
	previousArrivalAt := departureAt
	for index := 0; index < segmentCount; index++ {
		fmt.Fprintf(output, "\n--- Trecho %d: %s → ? ---\n", index+1, currentOrigin)
		destination, ok := chooseDifferentCity(input, output, currentOrigin, "Destino")
		if !ok {
			return true
		}

		segmentDepartureAt := previousArrivalAt
		if index == 0 {
			fmt.Fprintf(output, "Saída do primeiro trecho: %s\n", departureAt.Format(rideDateTimeLayout))
		} else {
			fmt.Fprintf(output, "Saída automática: %s\n", segmentDepartureAt.Format(rideDateTimeLayout))
		}

		segmentArrivalAt, ok := readDateTimeAfter(input, output, segmentDepartureAt,
			"Data e horário de chegada do trecho (dd/mm/aaaa hh:mm): ")
		if !ok {
			return true
		}

		priceCents, ok := readNonNegativeInt(input, output, "Preço em centavos (ex.: 2550 para R$ 25,50): ")
		if !ok {
			return true
		}
		availableSeats, ok := readPositiveInt(input, output, "Assentos disponíveis: ")
		if !ok {
			return true
		}

		segments = append(segments, protocol.CreateStageRequest{
			Origin:         currentOrigin,
			Destination:    destination,
			DepartureAt:    segmentDepartureAt,
			ArrivalAt:      segmentArrivalAt,
			PriceCents:     priceCents,
			AvailableSeats: availableSeats,
		})
		currentOrigin = destination
		previousArrivalAt = segmentArrivalAt
	}

	printRideSummary(output, departureAt, segments)
	confirmation, err := readLine(input, output, "Publicar esta carona? (s/N): ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler a confirmação.")
		return true
	}
	if !strings.EqualFold(confirmation, "s") {
		fmt.Fprintln(output, "Publicação cancelada.")
		return true
	}

	payload, err := json.Marshal(protocol.CreateRideRequest{
		DepartureAt: departureAt,
		Segments:    segments,
	})
	if err != nil {
		fmt.Fprintln(output, "Não foi possível preparar a publicação da carona.")
		return true
	}

	response, err := tcpClient.Send(protocol.Request{Action: protocol.ActionCreateRide, Payload: payload})
	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return false
	}
	if response.Success != "success" {
		fmt.Fprintln(output, response.Message)
		return true
	}

	var ride models.Ride
	if err := json.Unmarshal(response.Payload, &ride); err != nil {
		fmt.Fprintln(output, "O servidor retornou uma resposta inválida.")
		return true
	}

	fmt.Fprintln(output, response.Message)
	printRide(output, ride)
	return true
}

// listMyRides solicita ao servidor a lista de caronas publicadas pelo motorista autenticado e as exibe no terminal.
// Retorna um booleano indicando se a conexão com o servidor foi mantida.
func listMyRides(tcpClient *TCPClient, output io.Writer) bool {
	rides, ok := getMyRides(tcpClient, output)
	if !ok {
		return false
	}
	if len(rides) == 0 {
		fmt.Fprintln(output, "Você ainda não publicou caronas.")
		return true
	}

	for _, ride := range rides {
		printRide(output, ride)
	}
	return true
}

func getMyRides(tcpClient *TCPClient, output io.Writer) ([]models.Ride, bool) {
	response, err := tcpClient.Send(protocol.Request{Action: protocol.ActionListMyRides})
	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return nil, false
	}
	if response.Success != "success" {
		fmt.Fprintln(output, response.Message)
		return nil, true
	}

	var rides []models.Ride
	if err := json.Unmarshal(response.Payload, &rides); err != nil {
		fmt.Fprintln(output, "O servidor retornou uma resposta inválida.")
		return nil, true
	}

	fmt.Fprintln(output, response.Message)
	return rides, true
}

func cancelRide(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) bool {
	ride, ok := chooseRide(tcpClient, input, output, true, "Cancelar carona")
	if !ok || ride == nil {
		return true
	}

	payload, err := json.Marshal(protocol.CancelRideRequest{RideID: ride.ID})
	if err != nil {
		fmt.Fprintln(output, "Não foi possível preparar o cancelamento.")
		return true
	}

	response, err := tcpClient.Send(protocol.Request{Action: protocol.ActionCancelRide, Payload: payload})
	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return false
	}

	fmt.Fprintln(output, response.Message)
	return true
}

func listRidePassengers(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) bool {
	ride, ok := chooseRide(tcpClient, input, output, false, "Consultar passageiros")
	if !ok || ride == nil {
		return true
	}

	payload, err := json.Marshal(protocol.GetRidePassengersRequest{RideID: ride.ID})
	if err != nil {
		fmt.Fprintln(output, "Não foi possível preparar a consulta.")
		return true
	}

	response, err := tcpClient.Send(protocol.Request{Action: protocol.ActionGetRidePassengers, Payload: payload})
	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return false
	}
	if response.Success != "success" {
		fmt.Fprintln(output, response.Message)
		return true
	}

	var result protocol.RidePassengersResponse
	if err := json.Unmarshal(response.Payload, &result); err != nil {
		fmt.Fprintln(output, "O servidor retornou uma resposta inválida.")
		return true
	}

	fmt.Fprintln(output, response.Message)
	for index, segment := range result.Segments {
		fmt.Fprintf(output, "\n%d. %s → %s (%s até %s)\n", index+1, segment.Origin, segment.Destination,
			segment.DepartureAt.Format(rideDateTimeLayout), segment.ArrivalAt.Format(rideDateTimeLayout))
		if len(segment.Passengers) == 0 {
			fmt.Fprintln(output, "Nenhum passageiro confirmado neste trecho.")
			continue
		}
		for _, passenger := range segment.Passengers {
			fmt.Fprintf(output, "- %s <%s>\n", passenger.Name, passenger.Email)
		}
	}

	return true
}

func chooseRide(tcpClient *TCPClient, input *bufio.Reader, output io.Writer, onlyActive bool, action string) (*models.Ride, bool) {
	rides, connected := getMyRides(tcpClient, output)
	if !connected {
		return nil, false
	}

	availableRides := make([]models.Ride, 0, len(rides))
	for _, ride := range rides {
		if !onlyActive || !ride.Cancelled {
			availableRides = append(availableRides, ride)
		}
	}
	if len(availableRides) == 0 {
		if onlyActive {
			fmt.Fprintln(output, "Você não possui caronas ativas para cancelar.")
		} else {
			fmt.Fprintln(output, "Você ainda não publicou caronas para consultar.")
		}
		return nil, true
	}

	fmt.Fprintf(output, "\n=== %s ===\n", action)
	for index, ride := range availableRides {
		fmt.Fprintf(output, "%d - %s → %s | %s\n", index+1, ride.Segments[0].Origin,
			ride.Segments[len(ride.Segments)-1].Destination, ride.DepartureAt.Format(rideDateTimeLayout))
	}

	choice, ok := chooseNumber(input, output, "Escolha uma carona ou 0 para voltar: ", len(availableRides))
	if !ok || choice == 0 {
		return nil, ok
	}

	return &availableRides[choice-1], true
}

func chooseCity(input *bufio.Reader, output io.Writer, label string) (enum.City, bool) {
	fmt.Fprintln(output, "Cidades disponíveis:")
	for _, city := range enum.Cities() {
		fmt.Fprintf(output, "%d - %s\n", city, city)
	}

	for {
		choice, err := readLine(input, output, label+": ")
		if err != nil {
			fmt.Fprintln(output, "Não foi possível ler a cidade.")
			return enum.CityUnknown, false
		}

		value, err := strconv.Atoi(choice)
		city := enum.City(value)
		if err != nil || !city.IsValid() {
			fmt.Fprintln(output, "Cidade inválida. Escolha uma das opções exibidas.")
			continue
		}

		return city, true
	}
}

func chooseDifferentCity(input *bufio.Reader, output io.Writer, origin enum.City, label string) (enum.City, bool) {
	for {
		city, ok := chooseCity(input, output, label)
		if !ok {
			return enum.CityUnknown, false
		}
		if city == origin {
			fmt.Fprintln(output, "O destino deve ser diferente da origem do trecho.")
			continue
		}
		return city, true
	}
}

func readDateTime(input *bufio.Reader, output io.Writer, prompt string) (time.Time, bool) {
	for {
		text, err := readLine(input, output, prompt)
		if err != nil {
			fmt.Fprintln(output, "Não foi possível ler a data e horário.")
			return time.Time{}, false
		}

		dateTime, err := time.ParseInLocation(rideDateTimeLayout, text, time.Local)
		if err != nil {
			fmt.Fprintln(output, "Data e horário inválidos. Use o formato dd/mm/aaaa hh:mm.")
			continue
		}

		return dateTime, true
	}
}

func readNonNegativeInt(input *bufio.Reader, output io.Writer, prompt string) (int, bool) {
	for {
		text, err := readLine(input, output, prompt)
		if err != nil {
			fmt.Fprintln(output, "Não foi possível ler o valor.")
			return 0, false
		}

		value, err := strconv.Atoi(text)
		if err != nil || value < 0 {
			fmt.Fprintln(output, "O valor deve ser um número inteiro não negativo.")
			continue
		}

		return value, true
	}
}

func readPositiveInt(input *bufio.Reader, output io.Writer, prompt string) (int, bool) {
	for {
		value, ok := readNonNegativeInt(input, output, prompt)
		if !ok {
			return 0, false
		}
		if value == 0 {
			fmt.Fprintln(output, "O valor deve ser maior que zero.")
			continue
		}
		return value, true
	}
}

func readDateTimeAfter(input *bufio.Reader, output io.Writer, reference time.Time, prompt string) (time.Time, bool) {
	for {
		dateTime, ok := readDateTime(input, output, prompt)
		if !ok {
			return time.Time{}, false
		}
		if !dateTime.After(reference) {
			fmt.Fprintln(output, "A chegada deve ocorrer depois da saída do trecho.")
			continue
		}
		return dateTime, true
	}
}

func printRideSummary(output io.Writer, departureAt time.Time, segments []protocol.CreateStageRequest) {
	fmt.Fprintln(output, "\n=== Resumo da carona ===")
	fmt.Fprintf(output, "Partida: %s\n", departureAt.Format(rideDateTimeLayout))
	for index, segment := range segments {
		fmt.Fprintf(output, "%d. %s → %s | %s até %s | %s | %d assento(s)\n",
			index+1,
			segment.Origin,
			segment.Destination,
			segment.DepartureAt.Format(rideDateTimeLayout),
			segment.ArrivalAt.Format(rideDateTimeLayout),
			formatCents(segment.PriceCents),
			segment.AvailableSeats,
		)
	}
}

func printRide(output io.Writer, ride models.Ride) {
	status := "Ativa"
	if ride.Cancelled {
		status = "Cancelada"
	}

	fmt.Fprintf(output, "\nID: %s\nPartida: %s\nStatus: %s\n", ride.ID, ride.DepartureAt.Format(rideDateTimeLayout), status)
	for index, segment := range ride.Segments {
		fmt.Fprintf(output, "%d. %s → %s | %s até %s | %s | %d assento(s) disponível(is)\n",
			index+1,
			segment.Origin,
			segment.Destination,
			segment.DepartureAt.Format(rideDateTimeLayout),
			segment.ArrivalAt.Format(rideDateTimeLayout),
			formatCents(segment.PriceCents),
			segment.AvailableSeats,
		)
	}
	fmt.Fprintf(output, "Preço total da rota: %s\n", formatCents(ride.TotalPriceCents()))
}

func formatCents(value int) string {
	return fmt.Sprintf("R$ %d,%02d", value/100, value%100)
}
