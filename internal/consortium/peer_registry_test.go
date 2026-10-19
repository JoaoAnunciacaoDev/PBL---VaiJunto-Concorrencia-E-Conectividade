package consortium

import (
	"testing"
)

func TestParsePeers(t *testing.T) {
	raw := "empresa-b=http://empresa-b:8081, empresa-c=http://empresa-c:8081/"
	peers, err := ParsePeers(raw)
	if err != nil {
		t.Fatalf("erro ao parsear peers válidos: %v", err)
	}

	if len(peers) != 2 {
		t.Fatalf("esperado 2 peers, obtido %d", len(peers))
	}

	if peers[0].CompanyID != "empresa-b" || peers[0].InternalURL != "http://empresa-b:8081" {
		t.Fatalf("peer 0 incorreto: %+v", peers[0])
	}
	if peers[1].CompanyID != "empresa-c" || peers[1].InternalURL != "http://empresa-c:8081" {
		t.Fatalf("peer 1 incorreto: %+v", peers[1])
	}
}

func TestParsePeersEmpty(t *testing.T) {
	peers, err := ParsePeers("  ")
	if err != nil || len(peers) != 0 {
		t.Fatalf("string vazia deveria retornar lista vazia sem erro")
	}
}

func TestParsePeersInvalidFormat(t *testing.T) {
	_, err := ParsePeers("empresa-b-sem-igual")
	if err == nil {
		t.Fatal("esperado erro para formato sem '='")
	}
}

func TestPeerRegistry(t *testing.T) {
	peers := []Peer{
		{CompanyID: "empresa-c", InternalURL: "http://c:8081"},
		{CompanyID: "empresa-a", InternalURL: "http://a:8081"},
	}
	reg := NewPeerRegistry(peers)

	all := reg.All()
	if len(all) != 2 {
		t.Fatalf("esperado 2 peers, obtido %d", len(all))
	}
	// Deve vir ordenado por CompanyID
	if all[0].CompanyID != "empresa-a" || all[1].CompanyID != "empresa-c" {
		t.Fatalf("ordenação incorreta: %+v", all)
	}

	p, exists := reg.Get("empresa-a")
	if !exists || p.InternalURL != "http://a:8081" {
		t.Fatalf("Get para empresa-a falhou: %+v", p)
	}

	_, exists = reg.Get("empresa-desconhecida")
	if exists {
		t.Fatal("empresa inexistente não deveria existir no registry")
	}
}
