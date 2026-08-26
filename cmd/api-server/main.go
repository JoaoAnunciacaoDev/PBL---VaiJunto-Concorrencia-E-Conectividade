package main

import (
	"log"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/server"
)

func main() {
	srv := server.NewServer(":8080")

	if err := srv.Start(); err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
}