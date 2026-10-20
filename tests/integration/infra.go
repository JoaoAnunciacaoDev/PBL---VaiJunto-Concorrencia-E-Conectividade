package integration

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/consortium"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/server"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// TestNode encapsula uma instância de servidor executando em-processo
// com listeners de rede TCP e HTTP reais em portas dinâmicas de loopback.
type TestNode struct {
	CompanyID string
	Server    *server.Server
	DataDir   string
	UsersPath string
	LockTTL   time.Duration
}

// TestCluster gerencia um conjunto de empresas do consórcio VaiJunto.
type TestCluster struct {
	t       *testing.T
	mu      sync.Mutex
	Nodes   map[string]*TestNode
	lockTTL time.Duration
}

// NewTestCluster inicializa nós para empresa-a, empresa-b e empresa-c,
// interconecta seus registros de peers e aguarda a inicialização.
func NewTestCluster(t *testing.T, lockTTL time.Duration) *TestCluster {
	t.Helper()

	if lockTTL <= 0 {
		lockTTL = 2 * time.Second
	}

	cluster := &TestCluster{
		t:       t,
		Nodes:   make(map[string]*TestNode),
		lockTTL: lockTTL,
	}

	companyIDs := []string{"empresa-a", "empresa-b", "empresa-c"}

	// 1. Cria e inicia cada nó isolado
	for _, companyID := range companyIDs {
		node := cluster.startNewNode(companyID, lockTTL)
		cluster.Nodes[companyID] = node
	}

	// 2. Interconecta todos os peers mutuamente
	cluster.reconnectAllPeers()

	// 3. Garante que todos os nós respondem ao health check
	for _, node := range cluster.Nodes {
		cluster.waitForHealth(node)
	}

	t.Cleanup(func() {
		cluster.StopAll()
	})

	return cluster
}

func (c *TestCluster) startNewNode(companyID string, lockTTL time.Duration) *TestNode {
	c.t.Helper()

	tempDir := c.t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		c.t.Fatalf("[%s] criar dataDir: %v", companyID, err)
	}
	usersPath := filepath.Join(dataDir, "users.json")

	srv, err := server.NewServerWithConfig(server.ServerConfig{
		CompanyID:   companyID,
		TCPAddr:     "127.0.0.1:0",
		HTTPAddr:    "127.0.0.1:0",
		UsersPath:   usersPath,
		LockTTL:     lockTTL,
		HTTPTimeout: 2 * time.Second,
	})
	if err != nil {
		c.t.Fatalf("[%s] criar servidor: %v", companyID, err)
	}

	go func() {
		_ = srv.Start()
	}()

	// Aguarda os listeners TCP e HTTP estarem vinculados
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if srv.TCPAddr() != "127.0.0.1:0" && srv.HTTPAddr() != "127.0.0.1:0" && srv.TCPAddr() != "" && srv.HTTPAddr() != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	return &TestNode{
		CompanyID: companyID,
		Server:    srv,
		DataDir:   dataDir,
		UsersPath: usersPath,
		LockTTL:   lockTTL,
	}
}

func (c *TestCluster) reconnectAllPeers() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for myID, myNode := range c.Nodes {
		for peerID, peerNode := range c.Nodes {
			if myID == peerID {
				continue
			}
			myNode.Server.Peers().Add(consortium.Peer{
				CompanyID:   peerID,
				InternalURL: "http://" + peerNode.Server.HTTPAddr(),
			})
		}
	}
}

func (c *TestCluster) waitForHealth(node *TestNode) {
	c.t.Helper()
	client := &http.Client{Timeout: 500 * time.Millisecond}
	url := fmt.Sprintf("http://%s/internal/v1/health", node.Server.HTTPAddr())

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			return
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}
	c.t.Fatalf("[%s] timeout aguardando health check em %s", node.CompanyID, url)
}

// StopNode encerra o servidor especificado, simulando queda ou interrupção de processo.
func (c *TestCluster) StopNode(companyID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, exists := c.Nodes[companyID]
	if !exists {
		return
	}
	_ = node.Server.Close()
}

// RestartNode reinicia um nó previamente parado com os mesmos arquivos de dados e WAL.
func (c *TestCluster) RestartNode(companyID string) *TestNode {
	c.t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()

	oldNode, exists := c.Nodes[companyID]
	if !exists {
		c.t.Fatalf("[%s] nó não encontrado no cluster", companyID)
	}

	srv, err := server.NewServerWithConfig(server.ServerConfig{
		CompanyID:   companyID,
		TCPAddr:     "127.0.0.1:0",
		HTTPAddr:    "127.0.0.1:0",
		UsersPath:   oldNode.UsersPath,
		LockTTL:     oldNode.LockTTL,
		HTTPTimeout: 2 * time.Second,
	})
	if err != nil {
		c.t.Fatalf("[%s] erro ao reiniciar servidor: %v", companyID, err)
	}

	go func() {
		_ = srv.Start()
	}()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if srv.TCPAddr() != "127.0.0.1:0" && srv.HTTPAddr() != "127.0.0.1:0" && srv.TCPAddr() != "" && srv.HTTPAddr() != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	newNode := &TestNode{
		CompanyID: companyID,
		Server:    srv,
		DataDir:   oldNode.DataDir,
		UsersPath: oldNode.UsersPath,
		LockTTL:   oldNode.LockTTL,
	}
	c.Nodes[companyID] = newNode

	// Reconecta peers
	for peerID, peerNode := range c.Nodes {
		if companyID == peerID {
			continue
		}
		newNode.Server.Peers().Add(consortium.Peer{
			CompanyID:   peerID,
			InternalURL: "http://" + peerNode.Server.HTTPAddr(),
		})
		peerNode.Server.Peers().Add(consortium.Peer{
			CompanyID:   companyID,
			InternalURL: "http://" + newNode.Server.HTTPAddr(),
		})
	}

	c.waitForHealth(newNode)
	return newNode
}

// StopAll encerra todos os nós do cluster graciosamente.
func (c *TestCluster) StopAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, node := range c.Nodes {
		_ = node.Server.Close()
	}
}

// TestTCPClient encapsula uma conexão TCP de cliente.
type TestTCPClient struct {
	conn    net.Conn
	encoder *json.Encoder
	decoder *json.Decoder
}

func ConnectTestTCPClient(tcpAddr string) (*TestTCPClient, error) {
	conn, err := net.DialTimeout("tcp", tcpAddr, 2*time.Second)
	if err != nil {
		return nil, err
	}
	return &TestTCPClient{
		conn:    conn,
		encoder: json.NewEncoder(conn),
		decoder: json.NewDecoder(conn),
	}, nil
}

func (c *TestTCPClient) Close() error {
	return c.conn.Close()
}

func (c *TestTCPClient) Send(action protocol.Action, payload any) (protocol.Response, error) {
	_ = c.conn.SetDeadline(time.Now().Add(4 * time.Second))
	data, err := json.Marshal(payload)
	if err != nil {
		return protocol.Response{}, err
	}
	if err := protocol.SendJson(c.encoder, protocol.Request{Action: action, Payload: data}); err != nil {
		return protocol.Response{}, err
	}
	var res protocol.Response
	if err := protocol.ReadJson(c.decoder, &res); err != nil {
		return protocol.Response{}, err
	}
	return res, nil
}

// RegisterAndLoginPassenger cria e autentica um passageiro via TCP.
func RegisterAndLoginPassenger(tcpAddr, name, email, password string) (*TestTCPClient, error) {
	client, err := ConnectTestTCPClient(tcpAddr)
	if err != nil {
		return nil, err
	}

	regPayload := protocol.CreateUserRequest{
		Name:     name,
		Email:    email,
		Password: password,
		Role:     models.RolePassenger,
	}
	regRes, err := client.Send(protocol.ActionRegisterUser, regPayload)
	if err != nil || regRes.Success != "success" {
		_ = client.Close()
		return nil, fmt.Errorf("cadastro falhou: err=%v res=%+v", err, regRes)
	}

	loginPayload := protocol.LoginRequest{
		Email:    email,
		Password: password,
	}
	loginRes, err := client.Send(protocol.ActionLogin, loginPayload)
	if err != nil || loginRes.Success != "success" {
		_ = client.Close()
		return nil, fmt.Errorf("login falhou: err=%v res=%+v", err, loginRes)
	}

	return client, nil
}

// CreateTestRide cadastra motorista, veículo e publica uma carona de teste diretamente no repositório.
func CreateTestRide(
	repo *server.Repository,
	driverEmail string,
	origin, dest enum.City,
	departure time.Time,
	seats int,
	priceCents int,
) (*models.Ride, error) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("Senha@123"), bcrypt.MinCost)
	driverUser := &models.User{
		ID:           uuid.New(),
		Name:         "Motorista Teste",
		Email:        driverEmail,
		PasswordHash: string(hash),
		Role:         models.RoleDriver,
	}
	if err := repo.SaveUser(driverUser); err != nil {
		return nil, fmt.Errorf("salvar motorista: %w", err)
	}

	driver := &models.Driver{
		User: *driverUser,
		Vehicle: &models.Vehicle{
			Plate:        "ABC1D23",
			Model:        "Sedan Teste",
			Color:        "Preto",
			SeatCapacity: seats + 2,
		},
	}
	if err := repo.SaveDriver(driver); err != nil {
		return nil, fmt.Errorf("salvar veículo motorista: %w", err)
	}

	ride := &models.Ride{
		ID:          uuid.New(),
		DriverID:    driver.ID,
		DepartureAt: departure,
		Segments: []models.Stage{
			{
				ID:             uuid.New(),
				Origin:         origin,
				Destination:    dest,
				DepartureAt:    departure,
				ArrivalAt:      departure.Add(2 * time.Hour),
				PriceCents:     priceCents,
				AvailableSeats: seats,
			},
		},
	}

	if err := repo.SaveRide(ride); err != nil {
		return nil, fmt.Errorf("salvar carona: %w", err)
	}

	return ride, nil
}
