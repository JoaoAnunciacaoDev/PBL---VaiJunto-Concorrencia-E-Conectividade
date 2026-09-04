package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/utils"
	"github.com/google/uuid"
)

const rideDateTimeLayout = "02/01/2006 15:04"

func rideMenu(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) MenuResult {
	for {
		fmt.Fprintln(output, "\n=== Caronas ===")
		fmt.Fprintln(output, "1 - Publicar carona")
		fmt.Fprintln(output, "2 - Minhas caronas")
		fmt.Fprintln(output, "3 - Cancelar carona")
		fmt.Fprintln(output, "0 - Voltar")

		choice, err := readLine(input, output, "Opção: ")
		if err != nil {
			return MenuBack
		}

		switch choice {
		case "1":
			utils.ClearTerminal()
			if !createRide(tcpClient, input, output) {
				return MenuDisconnected
			}
		case "2":
			utils.ClearTerminal()
			if !listMyRides(tcpClient, output) {
				return MenuDisconnected
			}
		case "3":
			utils.ClearTerminal()
			if !cancelRide(tcpClient, input, output) {
				return MenuDisconnected
			}
		case "0":
			return MenuBack
		default:
			fmt.Fprintln(output, "Opção inválida.")
		}
	}
}

func createRide(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) bool {
	departureText, err := readLine(input, output, "Data e horário de partida (dd/mm/aaaa hh:mm): ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler a data de partida.")
		return true
	}

	departureAt, err := time.ParseInLocation(rideDateTimeLayout, departureText, time.Local)
	if err != nil {
		fmt.Fprintln(output, "Data e horário inválidos.")
		return true
	}

	segmentCountText, err := readLine(input, output, "Quantidade de trechos: ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler a quantidade de trechos.")
		return true
	}
	segmentCount, err := strconv.Atoi(segmentCountText)
	if err != nil || segmentCount <= 0 {
		fmt.Fprintln(output, "A quantidade de trechos deve ser um número inteiro maior que zero.")
		return true
	}

	segments := make([]protocol.CreateStageRequest, 0, segmentCount)
	for index := 0; index < segmentCount; index++ {
		fmt.Fprintf(output, "\n--- Trecho %d ---\n", index+1)

		origin, ok := chooseCity(input, output, "Origem")
		if !ok {
			return true
		}
		destination, ok := chooseCity(input, output, "Destino")
		if !ok {
			return true
		}

		segmentDepartureAt := departureAt
		if index == 0 {
			fmt.Fprintf(output, "Saída do primeiro trecho: %s\n", departureAt.Format(rideDateTimeLayout))
		} else {
			segmentDepartureAt, ok = readDateTime(input, output, "Data e horário de saída do trecho (dd/mm/aaaa hh:mm): ")
			if !ok {
				return true
			}
		}

		segmentArrivalAt, ok := readDateTime(input, output, "Data e horário de chegada do trecho (dd/mm/aaaa hh:mm): ")
		if !ok {
			return true
		}
		if !segmentArrivalAt.After(segmentDepartureAt) {
			fmt.Fprintln(output, "A chegada deve ocorrer depois da saída do trecho.")
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
			Origin:         origin,
			Destination:    destination,
			DepartureAt:    segmentDepartureAt,
			ArrivalAt:      segmentArrivalAt,
			PriceCents:     priceCents,
			AvailableSeats: availableSeats,
		})
	}

	payload, err := json.Marshal(protocol.CreateRideRequest{
		DepartureAt: departureAt,
		Segments:    segments,
	})
	if err != nil {
		fmt.Fprintln(output, "Não foi possível preparar a publicação da carona.")
		return true
	}

	response, err := tcpClient.Send(protocol.Request{Action: "create_ride", Payload: payload})
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

func listMyRides(tcpClient *TCPClient, output io.Writer) bool {
	response, err := tcpClient.Send(protocol.Request{Action: "list_my_rides"})
	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return false
	}
	if response.Success != "success" {
		fmt.Fprintln(output, response.Message)
		return true
	}

	var rides []models.Ride
	if err := json.Unmarshal(response.Payload, &rides); err != nil {
		fmt.Fprintln(output, "O servidor retornou uma resposta inválida.")
		return true
	}
	if len(rides) == 0 {
		fmt.Fprintln(output, "Você ainda não publicou caronas.")
		return true
	}

	fmt.Fprintln(output, response.Message)
	for _, ride := range rides {
		printRide(output, ride)
	}
	return true
}

func cancelRide(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) bool {
	rideIDText, err := readLine(input, output, "ID da carona: ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler o ID da carona.")
		return true
	}

	rideID, err := uuid.Parse(rideIDText)
	if err != nil {
		fmt.Fprintln(output, "ID da carona inválido.")
		return true
	}

	payload, err := json.Marshal(protocol.CancelRideRequest{RideID: rideID})
	if err != nil {
		fmt.Fprintln(output, "Não foi possível preparar o cancelamento.")
		return true
	}

	response, err := tcpClient.Send(protocol.Request{Action: "cancel_ride", Payload: payload})
	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return false
	}

	fmt.Fprintln(output, response.Message)
	return true
}

func chooseCity(input *bufio.Reader, output io.Writer, label string) (enum.City, bool) {
	fmt.Fprintln(output, "Cidades disponíveis:")
	for _, city := range enum.Cities() {
		fmt.Fprintf(output, "%d - %s\n", city, city)
	}

	choice, err := readLine(input, output, label+": ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler a cidade.")
		return enum.CityUnknown, false
	}

	value, err := strconv.Atoi(choice)
	if err != nil {
		fmt.Fprintln(output, "Cidade inválida.")
		return enum.CityUnknown, false
	}

	city := enum.City(value)
	if !city.IsValid() {
		fmt.Fprintln(output, "Cidade inválida.")
		return enum.CityUnknown, false
	}

	return city, true
}

func readPositiveInt(input *bufio.Reader, output io.Writer, prompt string) (int, bool) {
	value, ok := readNonNegativeInt(input, output, prompt)
	if !ok || value == 0 {
		if ok {
			fmt.Fprintln(output, "O valor deve ser maior que zero.")
		}
		return 0, false
	}

	return value, true
}

func readDateTime(input *bufio.Reader, output io.Writer, prompt string) (time.Time, bool) {
	text, err := readLine(input, output, prompt)
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler a data e horário.")
		return time.Time{}, false
	}

	dateTime, err := time.ParseInLocation(rideDateTimeLayout, text, time.Local)
	if err != nil {
		fmt.Fprintln(output, "Data e horário inválidos.")
		return time.Time{}, false
	}

	return dateTime, true
}

func readNonNegativeInt(input *bufio.Reader, output io.Writer, prompt string) (int, bool) {
	text, err := readLine(input, output, prompt)
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler o valor.")
		return 0, false
	}

	value, err := strconv.Atoi(text)
	if err != nil || value < 0 {
		fmt.Fprintln(output, "O valor deve ser um número inteiro não negativo.")
		return 0, false
	}

	return value, true
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
