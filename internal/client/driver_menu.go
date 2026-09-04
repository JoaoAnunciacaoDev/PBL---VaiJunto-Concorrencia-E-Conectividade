package client

import (
	"bufio"
	"fmt"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/utils"
	"io"
)

func driverMenu(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) MenuResult {
	for {
		fmt.Fprintln(output, "\n=== Terminal do Motorista ===")
		fmt.Fprintln(output, "1 - Veículo")
		fmt.Fprintln(output, "2 - Carona")
		fmt.Fprintln(output, "0 - Voltar")

		choice, err := readLine(input, output, "Opção: ")
		if err != nil {
			return MenuBack
		}

		switch choice {
		case "1":
			utils.ClearTerminal()
			if vehicleMenu(tcpClient, input, output) == MenuDisconnected {
				return MenuDisconnected
			}
		case "2":
			utils.ClearTerminal()
			if rideMenu(tcpClient, input, output) == MenuDisconnected {
				return MenuDisconnected
			}
		case "0":
			return MenuBack
		default:
			fmt.Fprintln(output, "Opção inválida.")
		}
	}
}
