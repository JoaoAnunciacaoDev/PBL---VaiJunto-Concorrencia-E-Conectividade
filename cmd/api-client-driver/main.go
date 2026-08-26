package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:8080")

	if err != nil {
		panic(err)
	}

	defer conn.Close()

	reader := bufio.NewReader(conn)

	reqDTO := protocol.CreateUserRequest{
		Name:     "Carlos Motorista",
		Email:    "carlos@email.com",
		Password: "123",
		Role:     models.RoleDriver,
	}

	payloadBytes, _ := json.Marshal(reqDTO)

	req := protocol.Request{
		Action:  "register_user",
		Payload: payloadBytes,
	}

	if err := protocol.SendJson(conn, req); err != nil {
		fmt.Println("Erro ao enviar:", err)
		return
	}

	var res protocol.Response
	if err := protocol.ReadJson(reader, &res); err != nil {
		fmt.Println("Erro ao ler resposta:", err)
		return
	}

	fmt.Printf("Resposta do Servidor: Sucesso=%v | Mensagem=%s\n", res.Success, res.Message)
}