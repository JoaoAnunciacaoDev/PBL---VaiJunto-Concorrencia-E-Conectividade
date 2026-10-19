package main

import (
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/consortium"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/server"
)

func main() {
	dataDirectoryFlag := flag.String("data-dir", "", "diretório onde os arquivos JSON serão armazenados")
	companyIDFlag := flag.String("company-id", "empresa-a", "identificador único da empresa no consórcio")
	tcpAddrFlag := flag.String("tcp-addr", ":8080", "endereço para escuta TCP dos clientes")
	httpAddrFlag := flag.String("http-addr", ":8081", "endereço para API REST inter-servidores")
	peersFlag := flag.String("peers", "", "lista de servidores parceiros (ex: empresa-b=http://b:8081,empresa-c=http://c:8081)")
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

	peers, err := consortium.ParsePeers(*peersFlag)
	if err != nil {
		log.Fatalf("Erro ao configurar servidores parceiros: %v", err)
	}

	srv, err := server.NewServerWithConfig(server.ServerConfig{
		CompanyID: *companyIDFlag,
		TCPAddr:   *tcpAddrFlag,
		HTTPAddr:  *httpAddrFlag,
		UsersPath: filepath.Join(dataDirectory, "users.json"),
		Peers:     peers,
	})

	if err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
	log.Printf("Empresa: %s | TCP: %s | HTTP: %s | Peers: %d | Dados em: %s",
		*companyIDFlag, *tcpAddrFlag, *httpAddrFlag, len(peers), dataDirectory)

	if err := srv.Start(); err != nil {
		log.Fatalf("Erro ao executar servidor: %v", err)
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
