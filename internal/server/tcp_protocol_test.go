package server

import (
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
)

func TestTCPRejectsMalformedMessagesAndKeepsServing(t *testing.T) {
	silenceServerLogs(t)
	server := newTestServer(t)
	server.readIdleTimeout = 75 * time.Millisecond
	server.writeTimeout = time.Second
	server.maxRequestSize = 256
	address := startTCPTestListener(t, server)

	testCases := []struct {
		name       string
		message    string
		closeWrite bool
	}{
		{name: "JSON truncado", message: `{"action":"login","payload":`, closeWrite: true},
		{name: "ação ausente", message: `{"payload":null}` + "\n"},
		{name: "campo desconhecido no envelope", message: `{"action":"login","payload":null,"unexpected":true}` + "\n"},
		{name: "conteúdo após o objeto", message: `{"action":"logout","payload":null} {}` + "\n"},
		{name: "payload inválido", message: `{"action":"login","payload":{"email":42,"password":[]}}` + "\n"},
		{name: "campo desconhecido no payload", message: `{"action":"login","payload":{"email":"a@example.com","password":"Senha@123","extra":true}}` + "\n"},
		{name: "mensagem excessivamente grande", message: `{"action":"login","payload":{"email":"` + strings.Repeat("a", 300) + `"}}` + "\n"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			response := sendRawTCPMessage(t, address, testCase.message, testCase.closeWrite)
			if response.Success != "error" {
				t.Fatalf("mensagem inválida deveria ser recusada: %+v", response)
			}
		})
	}

	// Depois de todos os clientes defeituosos, uma conexão válida ainda deve
	// ser processada normalmente pelo mesmo servidor.
	valid := `{"action":"register_user","payload":{"name":"Cliente Válido","email":"valido@example.com","password":"Senha@123","role":"PASSENGER"}}` + "\n"
	response := sendRawTCPMessage(t, address, valid, false)
	if response.Success != "success" {
		t.Fatalf("servidor deveria continuar atendendo clientes válidos: %+v", response)
	}
}

func TestTCPClosesIdleConnectionAfterDeadline(t *testing.T) {
	silenceServerLogs(t)
	server := newTestServer(t)
	server.readIdleTimeout = 50 * time.Millisecond
	server.writeTimeout = time.Second
	address := startTCPTestListener(t, server)

	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatalf("conectar: %v", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("definir prazo do teste: %v", err)
	}

	startedAt := time.Now()
	var response protocol.Response
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		t.Fatalf("ler resposta de timeout: %v", err)
	}
	if response.Success != "error" || !strings.Contains(strings.ToLower(response.Message), "tempo limite") {
		t.Fatalf("resposta de timeout inválida: %+v", response)
	}
	if elapsed := time.Since(startedAt); elapsed > 500*time.Millisecond {
		t.Fatalf("conexão ociosa demorou %s para ser encerrada", elapsed)
	}
}

func sendRawTCPMessage(t *testing.T, address, message string, closeWrite bool) protocol.Response {
	t.Helper()

	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatalf("conectar: %v", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("definir prazo: %v", err)
	}
	if _, err := conn.Write([]byte(message)); err != nil {
		t.Fatalf("enviar mensagem: %v", err)
	}
	if closeWrite {
		tcpConnection, ok := conn.(*net.TCPConn)
		if !ok {
			t.Fatal("conexão TCP esperada")
		}
		if err := tcpConnection.CloseWrite(); err != nil {
			t.Fatalf("encerrar escrita: %v", err)
		}
	}

	var response protocol.Response
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		t.Fatalf("ler resposta: %v", err)
	}
	return response
}
