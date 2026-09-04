package main

import (
	"log"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/server"
)

func main() {
	srv, err := server.NewServer(":8080", "data/users.json")

	if err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}

	if err := srv.Start(); err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
}
