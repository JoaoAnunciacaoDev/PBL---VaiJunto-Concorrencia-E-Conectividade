package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/utils"
)

func authenticatedMenu(tcpClient *TCPClient, input *bufio.Reader, output io.Writer, user protocol.UserResponse) enum.MenuResult {
	for {
		fmt.Fprintf(output, "\n=== Área de %s ===\n", user.Name)
		fmt.Fprintln(output, "1 - Ver meu perfil")
		fmt.Fprintln(output, "2 - Opções de passageiro")

		if user.Role == models.RoleDriver {
			fmt.Fprintln(output, "3 - Opções de motorista")
		}

		fmt.Fprintln(output, "0 - Logout")

		choice, err := readLine(input, output, "Opção: ")
		if err != nil {
			return enum.MenuBack
		}

		switch choice {
		case "1":
			utils.ClearTerminal()
			if !showMyProfile(tcpClient, output) {
				return enum.MenuDisconnected
			}

		case "2":
			utils.ClearTerminal()
			if passengerMenu(tcpClient, input, output) == enum.MenuDisconnected {
				return enum.MenuDisconnected
			}

		case "3":
			if user.Role != models.RoleDriver {
				utils.ClearTerminal()
				fmt.Fprintln(output, "Opção inválida.")
				continue
			}

			utils.ClearTerminal()
			if driverMenu(tcpClient, input, output) == enum.MenuDisconnected {
				return enum.MenuDisconnected
			}

		case "0":
			utils.ClearTerminal()
			if !logout(tcpClient, output) {
				return enum.MenuDisconnected
			}
			return enum.MenuBack

		default:
			utils.ClearTerminal()
			fmt.Fprintln(output, "Opção inválida.")
		}
	}
}

// showMyProfile solicita ao servidor as informações do perfil do usuário autenticado e as exibe no terminal.
// Retorna um booleano indicando se a conexão com o servidor foi mantida.
func showMyProfile(tcpClient *TCPClient, output io.Writer) bool {
	response, err := tcpClient.Send(protocol.Request{Action: "get_my_profile"})
	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return false
	}
	if response.Success != "success" {
		fmt.Fprintln(output, response.Message)
		return true
	}

	var user protocol.UserResponse
	if err := json.Unmarshal(response.Payload, &user); err != nil {
		fmt.Fprintln(output, "O servidor retornou uma resposta inválida.")
		return true
	}

	profile := "Passageiro"
	if user.Role == models.RoleDriver {
		profile = "Motorista"
	}
	fmt.Fprintf(output, "Nome: %s\nE-mail: %s\nPerfil: %s\n", user.Name, user.Email, profile)
	return true
}

// logout envia uma solicitação de logout ao servidor e exibe a mensagem de resposta no terminal.
// Retorna um booleano indicando se a conexão com o servidor foi mantida.
func logout(tcpClient *TCPClient, output io.Writer) bool {
	response, err := tcpClient.Send(protocol.Request{Action: "logout"})
	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return false
	}

	fmt.Fprintln(output, response.Message)
	return true
}
