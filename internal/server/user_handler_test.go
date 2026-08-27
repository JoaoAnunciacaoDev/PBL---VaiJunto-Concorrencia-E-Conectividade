package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"golang.org/x/crypto/bcrypt"
)

func TestRegisterUserAndLogin(t *testing.T) {
	server := newTestServer(t)
	registration := protocol.CreateUserRequest{
		Name:     "João Silva",
		Email:    "  JOAO@EXAMPLE.COM  ",
		Password: "Senha@123",
		Role:     models.RolePassenger,
	}

	response := sendRequest(t, server, "register_user", registration)
	if response.Success != "success" {
		t.Fatalf("cadastro deveria funcionar, recebeu: %s", response.Message)
	}

	user, err := server.repository.GetUserByEmail("joao@example.com")
	if err != nil {
		t.Fatalf("usuário deveria estar salvo: %v", err)
	}
	if user.PasswordHash == registration.Password {
		t.Fatal("a senha não deve ser salva em texto puro")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(registration.Password)); err != nil {
		t.Fatalf("o hash salvo deveria corresponder à senha: %v", err)
	}

	loginResponse := sendRequest(t, server, "login", protocol.LoginRequest{
		Email:    "JOAO@example.com",
		Password: "Senha@123",
	})
	if loginResponse.Success != "success" {
		t.Fatalf("login deveria funcionar, recebeu: %s", loginResponse.Message)
	}

	var loggedUser protocol.UserResponse
	if err := json.Unmarshal(loginResponse.Payload, &loggedUser); err != nil {
		t.Fatalf("resposta de login deveria conter usuário: %v", err)
	}
	if loggedUser.Email != "joao@example.com" {
		t.Errorf("e-mail normalizado = %q, esperado %q", loggedUser.Email, "joao@example.com")
	}
}

func TestRegisterUserRejectsInvalidEmailAndPassword(t *testing.T) {
	validRequest := protocol.CreateUserRequest{
		Name:     "Maria Souza",
		Email:    "maria@example.com",
		Password: "Senha@123",
		Role:     models.RolePassenger,
	}

	testCases := []struct {
		name   string
		mutate func(*protocol.CreateUserRequest)
	}{
		{
			name: "email inválido",
			mutate: func(request *protocol.CreateUserRequest) {
				request.Email = "maria.example.com"
			},
		},
		{
			name: "senha curta",
			mutate: func(request *protocol.CreateUserRequest) {
				request.Password = "Ab1@abc"
			},
		},
		{
			name: "senha sem letra",
			mutate: func(request *protocol.CreateUserRequest) {
				request.Password = "12345678!"
			},
		},
		{
			name: "senha sem número",
			mutate: func(request *protocol.CreateUserRequest) {
				request.Password = "Senha!!!"
			},
		},
		{
			name: "senha sem caractere especial",
			mutate: func(request *protocol.CreateUserRequest) {
				request.Password = "Senha123"
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := newTestServer(t)
			request := validRequest
			testCase.mutate(&request)

			response := sendRequest(t, server, "register_user", request)
			if response.Success != "error" {
				t.Fatalf("cadastro inválido deveria falhar, recebeu: %s", response.Message)
			}
		})
	}
}

func TestLoginRejectsWrongPasswordAndDuplicateEmail(t *testing.T) {
	server := newTestServer(t)
	registration := protocol.CreateUserRequest{
		Name:     "Ana Lima",
		Email:    "ana@example.com",
		Password: "Senha@123",
		Role:     models.RoleDriver,
	}

	if response := sendRequest(t, server, "register_user", registration); response.Success != "success" {
		t.Fatalf("cadastro inicial deveria funcionar, recebeu: %s", response.Message)
	}
	if response := sendRequest(t, server, "register_user", registration); response.Success != "error" {
		t.Fatalf("cadastro duplicado deveria falhar, recebeu: %s", response.Message)
	}

	response := sendRequest(t, server, "login", protocol.LoginRequest{
		Email:    registration.Email,
		Password: "Senha@321",
	})
	if response.Success != "error" {
		t.Fatalf("login com senha errada deveria falhar, recebeu: %s", response.Message)
	}
}

func TestSessionAllowsProfileOnlyAfterLoginAndClearsOnLogout(t *testing.T) {
	server := newTestServer(t)
	registration := protocol.CreateUserRequest{
		Name:     "Lucas Ramos",
		Email:    "lucas@example.com",
		Password: "Senha@123",
		Role:     models.RoleDriver,
	}
	if response := sendRequest(t, server, "register_user", registration); response.Success != "success" {
		t.Fatalf("cadastro deveria funcionar, recebeu: %s", response.Message)
	}

	session := &Session{}
	if response := sendRequestWithSession(t, server, session, "get_my_profile", nil); response.Success != "error" {
		t.Fatalf("perfil sem login deveria falhar, recebeu: %s", response.Message)
	}

	if response := sendRequestWithSession(t, server, session, "login", protocol.LoginRequest{
		Email:    registration.Email,
		Password: registration.Password,
	}); response.Success != "success" {
		t.Fatalf("login deveria funcionar, recebeu: %s", response.Message)
	}
	if !session.IsAuthenticated() {
		t.Fatal("login deveria autenticar a sessão")
	}

	profileResponse := sendRequestWithSession(t, server, session, "get_my_profile", nil)
	if profileResponse.Success != "success" {
		t.Fatalf("perfil autenticado deveria funcionar, recebeu: %s", profileResponse.Message)
	}

	var profile protocol.UserResponse
	if err := json.Unmarshal(profileResponse.Payload, &profile); err != nil {
		t.Fatalf("resposta do perfil deveria conter usuário: %v", err)
	}
	if profile.Email != registration.Email {
		t.Errorf("e-mail do perfil = %q, esperado %q", profile.Email, registration.Email)
	}

	if response := sendRequestWithSession(t, server, session, "logout", nil); response.Success != "success" {
		t.Fatalf("logout deveria funcionar, recebeu: %s", response.Message)
	}
	if session.IsAuthenticated() {
		t.Fatal("logout deveria limpar a sessão")
	}
	if response := sendRequestWithSession(t, server, session, "get_my_profile", nil); response.Success != "error" {
		t.Fatalf("perfil após logout deveria falhar, recebeu: %s", response.Message)
	}
}

func TestRegisterVehicleRequiresAuthenticatedDriver(t *testing.T) {
	server := newTestServer(t)
	driver := protocol.CreateUserRequest{
		Name:     "Marcos Lima",
		Email:    "marcos@example.com",
		Password: "Senha@123",
		Role:     models.RoleDriver,
	}
	passenger := protocol.CreateUserRequest{
		Name:     "Beatriz Alves",
		Email:    "beatriz@example.com",
		Password: "Senha@123",
		Role:     models.RolePassenger,
	}
	for _, user := range []protocol.CreateUserRequest{driver, passenger} {
		if response := sendRequest(t, server, "register_user", user); response.Success != "success" {
			t.Fatalf("cadastro deveria funcionar, recebeu: %s", response.Message)
		}
	}

	vehicle := protocol.CreateVehicleRequest{
		Plate:        "abc-1234",
		Model:        "Hatch",
		Color:        "Azul",
		SeatCapacity: 4,
	}
	if response := sendRequest(t, server, "register_vehicle", vehicle); response.Success != "error" {
		t.Fatalf("cadastro sem login deveria falhar, recebeu: %s", response.Message)
	}

	passengerSession := &Session{}
	if response := sendRequestWithSession(t, server, passengerSession, "login", protocol.LoginRequest{
		Email: passenger.Email, Password: passenger.Password,
	}); response.Success != "success" {
		t.Fatalf("login de passageiro deveria funcionar, recebeu: %s", response.Message)
	}
	if response := sendRequestWithSession(t, server, passengerSession, "register_vehicle", vehicle); response.Success != "error" {
		t.Fatalf("passageiro não deveria cadastrar veículo, recebeu: %s", response.Message)
	}

	driverSession := &Session{}
	if response := sendRequestWithSession(t, server, driverSession, "login", protocol.LoginRequest{
		Email: driver.Email, Password: driver.Password,
	}); response.Success != "success" {
		t.Fatalf("login de motorista deveria funcionar, recebeu: %s", response.Message)
	}
	response := sendRequestWithSession(t, server, driverSession, "register_vehicle", vehicle)
	if response.Success != "success" {
		t.Fatalf("motorista deveria cadastrar veículo, recebeu: %s", response.Message)
	}

	storedDriver, err := server.repository.GetDriverByUserID(driverSession.UserID)
	if err != nil {
		t.Fatalf("motorista deveria estar salvo: %v", err)
	}
	if storedDriver.Vehicle == nil {
		t.Fatal("motorista deveria possuir veículo")
	}
	if storedDriver.Vehicle.Plate != "ABC-1234" {
		t.Errorf("placa = %q, esperado %q", storedDriver.Vehicle.Plate, "ABC-1234")
	}
}

func TestRegisterUserPersistsDataAcrossServerRestart(t *testing.T) {
	usersPath := filepath.Join(t.TempDir(), "data", "users.json")
	if err := os.MkdirAll(filepath.Dir(usersPath), 0o755); err != nil {
		t.Fatalf("criar diretório de dados: %v", err)
	}
	if err := os.WriteFile(usersPath, nil, 0o644); err != nil {
		t.Fatalf("criar arquivo de usuários vazio: %v", err)
	}

	server := newTestServerAtPath(t, usersPath)
	registration := protocol.CreateUserRequest{
		Name:     "Pedro Santos",
		Email:    "pedro@example.com",
		Password: "Senha@123",
		Role:     models.RolePassenger,
	}

	if response := sendRequest(t, server, "register_user", registration); response.Success != "success" {
		t.Fatalf("cadastro deveria funcionar, recebeu: %s", response.Message)
	}

	content, err := os.ReadFile(usersPath)
	if err != nil {
		t.Fatalf("arquivo de usuários deveria existir: %v", err)
	}
	if strings.Contains(string(content), registration.Password) {
		t.Fatal("arquivo de usuários não deve guardar a senha em texto puro")
	}

	restartedServer := newTestServerAtPath(t, usersPath)
	response := sendRequest(t, restartedServer, "login", protocol.LoginRequest{
		Email:    registration.Email,
		Password: registration.Password,
	})
	if response.Success != "success" {
		t.Fatalf("login após reiniciar o servidor deveria funcionar, recebeu: %s", response.Message)
	}
}

func sendRequest(t *testing.T, server *Server, action string, payload any) protocol.Response {
	return sendRequestWithSession(t, server, &Session{}, action, payload)
}

func sendRequestWithSession(t *testing.T, server *Server, session *Session, action string, payload any) protocol.Response {
	t.Helper()

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("serializar payload: %v", err)
	}

	return server.processRequest(protocol.Request{Action: action, Payload: data}, session)
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	return newTestServerAtPath(t, filepath.Join(t.TempDir(), "data", "users.json"))
}

func newTestServerAtPath(t *testing.T, usersPath string) *Server {
	t.Helper()

	server, err := NewServer("", usersPath)
	if err != nil {
		t.Fatalf("criar servidor de teste: %v", err)
	}

	return server
}
