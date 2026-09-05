package main

import (
	"flag"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/server"
	"log"
	"os"
	"path/filepath"
)

func main() {
	dataDirectoryFlag := flag.String("data-dir", "", "diretório onde os arquivos JSON serão armazenados")
	flag.Parse()

	dataDirectory, err := resolveDataDirectory(*dataDirectoryFlag)
	if err != nil {
		log.Fatalf("Erro ao definir diretório de dados: %v", err)
	}

	srv, err := server.NewServer(":8080", filepath.Join(dataDirectory, "users.json"))

	if err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
	log.Printf("Dados persistidos em %s", dataDirectory)

	if err := srv.Start(); err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
}

// resolveDataDirectory usa o diretório informado ou procura a raiz do projeto
// (identificada por go.mod). Isso evita criar data/ dentro de cmd/ quando o
// servidor é iniciado a partir de um subdiretório durante o desenvolvimento.
func resolveDataDirectory(configuredDirectory string) (string, error) {
	if configuredDirectory != "" {
		return filepath.Abs(configuredDirectory)
	}

	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return filepath.Join(directory, "data"), nil
		}

		parent := filepath.Dir(directory)
		if parent == directory {
			return filepath.Join(directory, "data"), nil
		}
		directory = parent
	}
}
