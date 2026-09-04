package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/utils"
)

const searchDateLayout = "02/01/2006"

func passengerMenu(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) MenuResult {
	for {
		fmt.Fprintln(output, "\n=== Terminal do Passageiro ===")
		fmt.Fprintln(output, "1 - Buscar itinerários")
		fmt.Fprintln(output, "0 - Voltar")

		choice, err := readLine(input, output, "Opção: ")
		if err != nil {
			return MenuBack
		}

		switch choice {
		case "1":
			utils.ClearTerminal()
			if !searchItineraries(tcpClient, input, output) {
				return MenuDisconnected
			}
		case "0":
			return MenuBack
		default:
			fmt.Fprintln(output, "Opção inválida.")
		}
	}
}

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

	payload, err := json.Marshal(protocol.SearchItinerariesRequest{
		Origin:      origin,
		Destination: destination,
		Date:        date,
	})
	if err != nil {
		fmt.Fprintln(output, "Não foi possível preparar a busca.")
		return true
	}

	response, err := tcpClient.Send(protocol.Request{Action: "search_itineraries", Payload: payload})
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
		fmt.Fprintf(output, "\n=== Itinerário %d ===\n", index+1)
		fmt.Fprintf(output, "Partida: %s\nChegada: %s\n", itinerary.DepartureAt.Format(rideDateTimeLayout), itinerary.ArrivalAt.Format(rideDateTimeLayout))
		for _, segment := range itinerary.Segments {
			fmt.Fprintf(output, "%s → %s | %s até %s | %s\n",
				segment.Origin,
				segment.Destination,
				segment.DepartureAt.Format(rideDateTimeLayout),
				segment.ArrivalAt.Format(rideDateTimeLayout),
				formatCents(segment.PriceCents),
			)
		}
		fmt.Fprintf(output, "Preço total: %s\n", formatCents(itinerary.TotalPriceCents))
	}

	return true
}
