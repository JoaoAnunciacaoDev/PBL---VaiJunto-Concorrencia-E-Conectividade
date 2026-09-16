package main

import (
	"bufio"
	"flag"
	"log"
	"os"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/client"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
)

// main é o ponto de entrada do aplicativo cliente. Ele inicia a interface de terminal para interação com o usuário.
func main() {
	address := flag.String("addr", "localhost:8080", "endereço TCP do servidor no formato host:porta")
	flag.Parse()

	err := client.RunTerminal(
		*address,
		bufio.NewReader(os.Stdin),
		os.Stdout,
		client.TerminalOptions{
			Title:              "VaiJunto",
			DefaultRole:        models.RolePassenger,
			AllowRoleSelection: true,
		},
	)

	if err != nil {
		log.Fatal(err)
	}
}
