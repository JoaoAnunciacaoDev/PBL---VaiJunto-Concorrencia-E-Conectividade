package consortium

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Peer representa um outro servidor de empresa participante do consórcio.
type Peer struct {
	CompanyID   string `json:"company_id"`
	InternalURL string `json:"internal_url"`
}

// PeerRegistry gerencia os servidores remotos conhecidos no consórcio.
type PeerRegistry struct {
	mu    sync.RWMutex
	peers map[string]Peer
}

// NewPeerRegistry cria um novo registro a partir de uma lista inicial de peers.
func NewPeerRegistry(peers []Peer) *PeerRegistry {
	peerMap := make(map[string]Peer, len(peers))
	for _, peer := range peers {
		if peer.CompanyID != "" && peer.InternalURL != "" {
			peerMap[peer.CompanyID] = peer
		}
	}
	return &PeerRegistry{
		peers: peerMap,
	}
}

// All retorna todos os peers registrados ordenados por CompanyID.
func (r *PeerRegistry) All() []Peer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Peer, 0, len(r.peers))
	for _, peer := range r.peers {
		result = append(result, peer)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CompanyID < result[j].CompanyID
	})

	return result
}

// Get busca um peer pelo seu CompanyID.
func (r *PeerRegistry) Get(companyID string) (Peer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	peer, exists := r.peers[companyID]
	return peer, exists
}

// Add registra ou atualiza um peer no registry.
func (r *PeerRegistry) Add(peer Peer) {
	if peer.CompanyID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.peers[peer.CompanyID] = peer
}

// ParsePeers faz o parsing de uma string no formato:
// "empresa-b=http://empresa-b:8081,empresa-c=http://empresa-c:8081"
func ParsePeers(raw string) ([]Peer, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	entries := strings.Split(raw, ",")
	peers := make([]Peer, 0, len(entries))

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("formato de peer inválido %q (esperado nome=url)", entry)
		}

		companyID := strings.TrimSpace(parts[0])
		url := strings.TrimRight(strings.TrimSpace(parts[1]), "/")

		if companyID == "" || url == "" {
			return nil, fmt.Errorf("peer com id ou url vazios: %q", entry)
		}

		peers = append(peers, Peer{
			CompanyID:   companyID,
			InternalURL: url,
		})
	}

	return peers, nil
}
