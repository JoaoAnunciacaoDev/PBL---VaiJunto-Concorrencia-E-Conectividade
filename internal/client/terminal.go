package client

import (
	"bufio"
	"fmt"
	"io"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/utils"
)

type TerminalOptions struct {
	Title              string
	DefaultRole        models.UserRole
	AllowRoleSelection bool
}

func RunTerminal(address string, input *bufio.Reader, output io.Writer, options TerminalOptions) error {
	for {
		tcpClient, shouldExit, err := connectToServer(address, input, output)

		if err != nil {
			return err
		}

		if shouldExit {
			return nil
		}

		result := runGuestMenu(tcpClient, input, output, options)
		tcpClient.Close()

		if result == MenuDisconnected {
			utils.ClearTerminal()
			fmt.Fprintln(output, "Conexão com o servidor foi perdida.")
			continue
		}

		return nil
	}
}

func connectToServer(address string, input *bufio.Reader, output io.Writer) (*TCPClient, bool, error) {
	for {
		utils.ClearTerminal()

		fmt.Fprintf(output, "Tentando conectar ao servidor em %s...\n", address)
		tcpClient, err := Dial(address)

		if err == nil {
			fmt.Fprintln(output, "Conectado ao servidor.")
			return tcpClient, false, nil
		}

		fmt.Fprintf(output, "Não foi possível conectar ao servidor: %v\n", err)
		fmt.Fprintln(output, "1 - Tentar novamente")
		fmt.Fprintln(output, "0 - Sair")

		choice, readErr := readLine(input, output, "Opção: ")

		if readErr != nil {
			return nil, false, readErr
		}

		switch choice {
			case "1":
				utils.ClearTerminal()
			case "0":
				return nil, true, nil
			default:
				fmt.Fprintln(output, "Opção inválida.")
		}
	}
}
