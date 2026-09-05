package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/utils"
	"io"
	"strconv"
	"strings"
)

func vehicleMenu(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) enum.MenuResult {
	for {
		fmt.Fprintln(output, "\n=== Veículo ===")
		fmt.Fprintln(output, "1 - Cadastrar")
		fmt.Fprintln(output, "2 - Consultar")
		fmt.Fprintln(output, "3 - Atualizar")
		fmt.Fprintln(output, "4 - Remover")
		fmt.Fprintln(output, "0 - Voltar")

		choice, err := readLine(input, output, "Opção: ")
		if err != nil {
			return enum.MenuBack
		}

		switch choice {
		case "1":
			utils.ClearTerminal()
			if !registerVehicle(tcpClient, input, output) {
				return enum.MenuDisconnected
			}

		case "2":
			utils.ClearTerminal()
			if !showMyVehicle(tcpClient, output) {
				return enum.MenuDisconnected
			}

		case "3":
			utils.ClearTerminal()
			if !updateVehicle(tcpClient, input, output) {
				return enum.MenuDisconnected
			}

		case "4":
			utils.ClearTerminal()
			if !removeVehicle(tcpClient, input, output) {
				return enum.MenuDisconnected
			}

		case "0":
			return enum.MenuBack
			
		default:
			fmt.Fprintln(output, "Opção inválida.")
		}
	}
}

func registerVehicle(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) bool {
	return submitVehicle(tcpClient, input, output, protocol.ActionRegisterVehicle, "cadastro")
}

func updateVehicle(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) bool {
	return submitVehicle(tcpClient, input, output, protocol.ActionUpdateVehicle, "atualização")
}

// submitVehicle solicita ao usuário as informações do veículo, envia uma solicitação de cadastro ou atualização ao servidor e exibe a mensagem de resposta.
// Retorna um booleano indicando se a conexão com o servidor foi mantida.
func submitVehicle(tcpClient *TCPClient, input *bufio.Reader, output io.Writer, action protocol.Action, operation string) bool {
	plate, err := readLine(input, output, "Placa: ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler a placa.")
		return true
	}

	model, err := readLine(input, output, "Modelo: ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler o modelo.")
		return true
	}

	color, err := readLine(input, output, "Cor: ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler a cor.")
		return true
	}

	capacityText, err := readLine(input, output, "Quantidade de assentos: ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler a capacidade.")
		return true
	}

	seatCapacity, err := strconv.Atoi(capacityText)
	if err != nil {
		fmt.Fprintln(output, "A quantidade de assentos deve ser um número inteiro.")
		return true
	}

	payload, err := json.Marshal(protocol.CreateVehicleRequest{
		Plate:        plate,
		Model:        model,
		Color:        color,
		SeatCapacity: seatCapacity,
	})
	if err != nil {
		fmt.Fprintf(output, "Não foi possível preparar a %s do veículo.\n", operation)
		return true
	}

	response, err := tcpClient.Send(protocol.Request{Action: action, Payload: payload})
	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return false
	}

	fmt.Fprintln(output, response.Message)
	return true
}

func showMyVehicle(tcpClient *TCPClient, output io.Writer) bool {
	response, err := tcpClient.Send(protocol.Request{Action: protocol.ActionGetMyVehicle})
	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return false
	}

	if response.Success != "success" {
		fmt.Fprintln(output, response.Message)
		return true
	}

	var vehicle models.Vehicle
	if err := json.Unmarshal(response.Payload, &vehicle); err != nil {
		fmt.Fprintln(output, "O servidor retornou uma resposta inválida.")
		return true
	}

	fmt.Fprintf(output, "Placa: %s\nModelo: %s\nCor: %s\nAssentos: %d\n", vehicle.Plate, vehicle.Model, vehicle.Color, vehicle.SeatCapacity)
	return true
}

func removeVehicle(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) bool {
	confirmation, err := readLine(input, output, "Confirma a remoção do veículo? (s/N): ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler a confirmação.")
		return true
	}

	if strings.ToLower(confirmation) != "s" {
		fmt.Fprintln(output, "Remoção cancelada.")
		return true
	}

	response, err := tcpClient.Send(protocol.Request{Action: protocol.ActionRemoveVehicle})
	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return false
	}

	fmt.Fprintln(output, response.Message)
	return true
}
