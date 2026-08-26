package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/utils"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
)

type TerminalOptions struct {
	Title              string
	DefaultRole        models.UserRole
	AllowRoleSelection bool
}

func RunTerminal(address string, input *bufio.Reader, output io.Writer, options TerminalOptions) error {
	tcpClient, err := Dial(address)

	if err != nil {
		return err
	}

	defer tcpClient.Close()

	for {
		fmt.Fprintf(output, "\n=== %s ===\n", options.Title)
		fmt.Fprintln(output, "1 - Cadastrar")
		fmt.Fprintln(output, "2 - Login")
		fmt.Fprintln(output, "0 - Sair")

		choice, err := readLine(input, output, "Opção: ")
		if err != nil {
			return err
		}

		switch choice {
		case "1":
			utils.ClearTerminal()
			registerUser(tcpClient, input, output, options)

		case "2":
			utils.ClearTerminal()
			user, loggedIn := login(tcpClient, input, output)

			if loggedIn {
				authenticatedMenu(input, output, user)
			}
		case "0":
			utils.ClearTerminal()
			fmt.Fprintln(output, "Conexão encerrada.")

			return nil

		default:
			fmt.Fprintln(output, "Opção inválida.")
		}
	}
}

func registerUser(tcpClient *TCPClient, input *bufio.Reader, output io.Writer, options TerminalOptions) {
	name, err := readLine(input, output, "Nome: ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler o nome.")
		return
	}

	email, err := readLine(input, output, "E-mail: ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler o e-mail.")
		return
	}

	password, err := readLine(input, output, "Senha: ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler a senha.")
		return
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
		return
	}

	response, err := tcpClient.Send(protocol.Request{Action: "register_user", Payload: payload})

	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return
	}

	fmt.Fprintln(output, response.Message)
}

func login(tcpClient *TCPClient, input *bufio.Reader, output io.Writer) (protocol.UserResponse, bool) {
	email, err := readLine(input, output, "E-mail: ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler o e-mail.")
		return protocol.UserResponse{}, false
	}

	password, err := readLine(input, output, "Senha: ")
	if err != nil {
		fmt.Fprintln(output, "Não foi possível ler a senha.")
		return protocol.UserResponse{}, false
	}

	payload, err := json.Marshal(protocol.LoginRequest{Email: email, Password: password})
	if err != nil {
		fmt.Fprintln(output, "Não foi possível preparar o login.")
		return protocol.UserResponse{}, false
	}

	response, err := tcpClient.Send(protocol.Request{Action: "login", Payload: payload})
	if err != nil {
		fmt.Fprintf(output, "Erro de comunicação: %v\n", err)
		return protocol.UserResponse{}, false
	}

	if response.Success != "success" {
		fmt.Fprintln(output, response.Message)
		return protocol.UserResponse{}, false
	}

	var user protocol.UserResponse
	if err := json.Unmarshal(response.Payload, &user); err != nil {
		fmt.Fprintln(output, "O servidor retornou uma resposta inválida.")
		return protocol.UserResponse{}, false
	}

	fmt.Fprintf(output, "%s, %s!\n", response.Message, user.Name)
	return user, true
}

func authenticatedMenu(input *bufio.Reader, output io.Writer, user protocol.UserResponse) {
	for {
		fmt.Fprintf(output, "\n=== Área de %s ===\n", user.Name)
		fmt.Fprintln(output, "1 - Ver meu perfil")

		if user.Role == models.RoleDriver {
			fmt.Fprintln(output, "2 - Opções de motorista (em desenvolvimento)")
		} else {
			fmt.Fprintln(output, "2 - Opções de passageiro (em desenvolvimento)")
		}

		fmt.Fprintln(output, "0 - Logout")

		choice, err := readLine(input, output, "Opção: ")
		if err != nil {
			return
		}

		switch choice {
		case "1":
			utils.ClearTerminal()

			perfil := "Passageiro"

			if user.Role == models.RoleDriver {
				perfil = "Motorista"
			}
			
			fmt.Fprintf(output, "Nome: %s\nE-mail: %s\nPerfil: %s\n", user.Name, user.Email, perfil)

		case "2":
			utils.ClearTerminal()
			fmt.Fprintln(output, "Esta funcionalidade ainda não foi implementada.")

		case "0":
			utils.ClearTerminal()
			fmt.Fprintln(output, "Logout realizado.")

			return
			
		default:
			fmt.Fprintln(output, "Opção inválida.")
		}
	}
}

func chooseRole(input *bufio.Reader, output io.Writer) models.UserRole {
	utils.ClearTerminal()

	fmt.Fprintln(output, "Perfil: \n1 - Passageiro \n2 - Motorista")
	choice, err := readLine(input, output, "Opção: ")

	if err == nil && choice == "2" {
		return models.RoleDriver
	}

	return models.RolePassenger
}

func readLine(input *bufio.Reader, output io.Writer, prompt string) (string, error) {
	fmt.Fprint(output, prompt)
	value, err := input.ReadString('\n')

	if err != nil && err != io.EOF {
		return "", err
	}

	return strings.TrimSpace(value), nil
}
