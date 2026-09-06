package main

import (
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/server"
)

func main() {
	dataDirectoryFlag := flag.String("data-dir", "", "diretório onde os arquivos JSON serão armazenados")
	flag.Parse()

	dataDirectory, err := resolveDataDirectory(*dataDirectoryFlag)
	if err != nil {
		log.Fatalf("Erro ao definir diretório de dados: %v", err)
	}
	logFile, err := configureLogger(filepath.Join(filepath.Dir(dataDirectory), "logs"))
	if err != nil {
		log.Fatalf("Erro ao configurar log: %v", err)
	}
	defer logFile.Close()

	srv, err := server.NewServer(":8080", filepath.Join(dataDirectory, "users.json"))

	if err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
	log.Printf("Dados persistidos em %s", dataDirectory)

	if err := srv.Start(); err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
}

// configureLogger mantém os logs visíveis no terminal e os acrescenta ao
// arquivo server.log. O arquivo não é apagado a cada reinicialização.
func configureLogger(logDirectory string) (*os.File, error) {
	if err := os.MkdirAll(logDirectory, 0o755); err != nil {
		return nil, err
	}

	logFile, err := os.OpenFile(filepath.Join(logDirectory, "server.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	return logFile, nil
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
