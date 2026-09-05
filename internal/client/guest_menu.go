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
)

func runGuestMenu(tcpClient *TCPClient, input *bufio.Reader, output io.Writer, options TerminalOptions) enum.MenuResult {
	for {
		fmt.Fprintf(output, "\n=== %s ===\n", options.Title)
		fmt.Fprintln(output, "1 - Cadastrar")
		fmt.Fprintln(output, "2 - Login")
		fmt.Fprintln(output, "0 - Sair")

		choice, err := readLine(input, output, "Opção: ")
		if err != nil {
			return enum.MenuExit
		}

		switch choice {
		case "1":
			utils.ClearTerminal()
			if !registerUser(tcpClient, input, output, options) {
				return enum.MenuDisconnected
			}

		case "2":
			utils.ClearTerminal()
			user, loggedIn, connected := login(tcpClient, input, output)
			if !connected {
				return enum.MenuDisconnected
			}

			if loggedIn {
				if authenticatedMenu(tcpClient, input, output, user) == enum.MenuDisconnected {
					return enum.MenuDisconnected
				}
			}

		case "0":
			utils.ClearTerminal()
			fmt.Fprintln(output, "Conexão encerrada.")

			return enum.MenuExit

		default:
			fmt.Fprintln(output, "Opção inválida.")
		}
	}
}

// registerUser realiza o processo de cadastro de um novo usuário.
// Retorna um booleano indicando se a conexão com o servidor foi mantida.
func registerUser(tcpClient *TCPClient, input *bufio.Reader, output io.Writer, options TerminalOptions) bool {
	name, err := readLine(input, output, "Nome: ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler o nome.")
		return true
	}

	email, err := readLine(input, output, "E-mail: ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler o e-mail.")
		return true
	}

	password, err := readLine(input, output, "Senha: ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler a senha.")
		return true
	}

	role := options.DefaultRole
	if options.AllowRoleSelection {
		role = chooseRole(input, output)
	}

	payload, err := json.Marshal(protocol.CreateUserRequest{
		Name:     name,
		Email:    email,
		Password: password,
		Role:     role,
	})

	if err != nil {
		fmt.Fprintln(output, "Não foi possível preparar o cadastro.")
		return true
	}

	response, err := tcpClient.Send(protocol.Request{Action: protocol.ActionRegisterUser, Payload: payload})

	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return false
	}

	fmt.Fprintln(output, response.Message)
	return true
}

// login realiza o processo de login do usuário.
// Retorna o usuário autenticado, um booleano indicando se o login foi bem-sucedido 
// e outro booleano indicando se a conexão com o servidor foi mantida.
func login(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) (protocol.UserResponse, bool, bool) {
	email, err := readLine(input, output, "E-mail: ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler o e-mail.")
		return protocol.UserResponse{}, false, true
	}

	password, err := readLine(input, output, "Senha: ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler a senha.")
		return protocol.UserResponse{}, false, true
	}

	payload, err := json.Marshal(protocol.LoginRequest{Email: email, Password: password})
	if err != nil {
		fmt.Fprintln(output, "Não foi possível preparar o login.")
		return protocol.UserResponse{}, false, true
	}

	response, err := tcpClient.Send(protocol.Request{Action: "login", Payload: payload})
	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return protocol.UserResponse{}, false, false
	}

	if response.Success != "success" {
		fmt.Fprintln(output, response.Message)
		return protocol.UserResponse{}, false, true
	}

	var user protocol.UserResponse
	if err := json.Unmarshal(response.Payload, &user); err != nil {
		fmt.Fprintln(output, "O servidor retornou uma resposta inválida.")
		return protocol.UserResponse{}, false, true
	}

	fmt.Fprintf(output, "%s, %s!\n", response.Message, user.Name)
	return user, true, true
}

// chooseRole permite ao usuário escolher entre os perfis de Passageiro e Motorista.
// Retorna o perfil escolhido pelo usuário.
func chooseRole(input *bufio.Reader, output io.Writer) models.UserRole {
	utils.ClearTerminal()

	fmt.Fprintln(output, "Perfil: \n1 - Passageiro \n2 - Motorista")
	choice, err := readLine(input, output, "Opção: ")

	if err == nil && choice == "2" {
		return models.RoleDriver
	}

	return models.RolePassenger
}
