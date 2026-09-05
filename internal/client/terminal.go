package client

import (
	"bufio"
	"fmt"
	"io"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/utils"
)

type TerminalOptions struct {
	Title              string
	DefaultRole        models.UserRole
	AllowRoleSelection bool
}

// RunTerminal inicia a interface de terminal para interação com o usuário.
// Ele tenta conectar ao servidor no endereço fornecido e, em caso de falha, oferece opções para tentar novamente ou sair.
// Retorna um erro se ocorrer algum problema durante a execução.
func RunTerminal(address string, input *bufio.Reader, output io.Writer, options TerminalOptions) error {
	for {
		// Tenta conectar ao servidor
		tcpClient, shouldExit, err := connectToServer(address, input, output)

		if err != nil {
			return err
		}

		if shouldExit {
			return nil
		}

		// Se a conexão for bem-sucedida, executa o menu do convidado
		result := runGuestMenu(tcpClient, input, output, options)
		tcpClient.Close() // Fechar a conexão TCP após sair do menu

		// Se a conexão for perdida, exibe uma mensagem e tenta reconectar
		if result == enum.MenuDisconnected {
			utils.ClearTerminal()
			fmt.Fprintln(output, "Conexão com o servidor foi perdida.")
			continue
		}

		return nil
	}
}

// connectToServer tenta estabelecer uma conexão TCP com o servidor no endereço fornecido.
// Retorna o cliente TCP, um booleano indicando se deve sair do programa e um erro, se houver.
func connectToServer(address string, input *bufio.Reader, output io.Writer) (*TCPClient, bool, error) {
	for {
		utils.ClearTerminal()

		fmt.Fprintf(output, "Tentando conectar ao servidor em %s...\n", address)
		tcpClient, err := Dial(address)

		// Se a conexão for bem-sucedida, retorna o cliente TCP e false para indicar que não deve sair
		if err == nil {
			fmt.Fprintln(output, "Conectado ao servidor.")
			return tcpClient, false, nil
		}

		// Se a conexão falhar, exibe uma mensagem de erro e oferece opções para tentar novamente ou sair
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
