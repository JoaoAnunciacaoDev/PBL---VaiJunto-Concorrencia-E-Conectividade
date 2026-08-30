package main

import (
	"bufio"
	"log"
	"os"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/client"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
)

func main() {
	err := client.RunTerminal(
		"localhost:8080",
		bufio.NewReader(os.Stdin),
		os.Stdout,
		client.TerminalOptions{
			Title:       "VaiJunto - Passageiro",
			DefaultRole: models.RolePassenger,
		},
	)

	if err != nil {
		log.Fatal(err)
	}
}
