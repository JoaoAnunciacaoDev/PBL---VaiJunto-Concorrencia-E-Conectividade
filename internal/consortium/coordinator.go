package consortium

import (
	"bytes"
	"strings"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/google/uuid"
)

// CoordinatorRepo é a interface do repositório local necessária para o Coordenador 2PC.
type CoordinatorRepo interface {
	GetRideByID(id uuid.UUID) (*models.Ride, error)
	DecrementSeat(rideID, segmentID uuid.UUID) error
	IncrementSeat(rideID, segmentID uuid.UUID) error
	DecrementSeatIdempotent(rideID, segmentID uuid.UUID, txID string) error
	IsTxApplied(txID string, segmentID uuid.UUID) bool
}

// Coordinator orquestra a confirmação de itinerários multitrecho via Two-Phase Commit (2PC).
type Coordinator struct {
	companyID   string
	clock       *LamportClock
	peers       *PeerRegistry
	wal         *WAL
	lockStore   *LockStore
	repo        CoordinatorRepo
	httpClient  *http.Client
	httpTimeout time.Duration
}

// NewCoordinator instancia um novo coordenador de transações distribuídas.
func NewCoordinator(
	companyID string,
	clock *LamportClock,
	peers *PeerRegistry,
	wal *WAL,
	lockStore *LockStore,
	repo CoordinatorRepo,
	httpTimeout time.Duration,
) *Coordinator {
	if httpTimeout <= 0 {
		httpTimeout = 3 * time.Second
	}

	return &Coordinator{
		companyID:   companyID,
		clock:       clock,
		peers:       peers,
		wal:         wal,
		lockStore:   lockStore,
		repo:        repo,
		httpClient:  &http.Client{Timeout: httpTimeout},
		httpTimeout: httpTimeout,
	}
}

type prepareResult struct {
	segment models.ReservedSegment
	ok      bool
	reason  string
}

// ConfirmItinerary executa o Two-Phase Commit atômico para reservar todos os trechos do itinerário.
// Ou todos os assentos são garantidos e confirmados, ou nenhum assento é reservado.
func (c *Coordinator) ConfirmItinerary(passengerID uuid.UUID, segments []models.ReservedSegment) error {
	if len(segments) == 0 {
		return errors.New("itinerário sem trechos")
	}

	txID := uuid.New().String()
	lamport := c.clock.Tick()

	segmentIDStrings := make([]string, 0, len(segments))
	walSegments := make([]WALSegment, 0, len(segments))
	for _, s := range segments {
		segmentIDStrings = append(segmentIDStrings, s.SegmentID.String())
		walSegments = append(walSegments, WALSegment{
			SegmentID: s.SegmentID,
			RideID:    s.RideID,
			CompanyID: s.CompanyID,
		})
	}

	log.Printf("[coordinator] iniciando 2PC tx_id=%s trechos=%d lamport=%d", txID, len(segments), lamport)

	// Anota o início da transação no WAL com metadados completos dos trechos
	if err := c.wal.Append(WALEntry{
		TxID:          txID,
		Role:          "coordinator",
		Status:        TxPrepared,
		CoordinatorID: c.companyID,
		SegmentIDs:    segmentIDStrings,
		Segments:      walSegments,
		UpdatedAt:     time.Now(),
	}); err != nil {
		return fmt.Errorf("falha ao persistir início da transação no WAL: %w", err)
	}

	// FASE 1: PREPARE
	var preparedSegments []models.ReservedSegment
	var prepareError error

	for _, segment := range segments {
		res := c.prepareSegment(txID, lamport, passengerID, segment)
		if !res.ok {
			prepareError = fmt.Errorf("trecho da empresa %s indisponível: %s", segment.CompanyID, res.reason)
			break
		}
		preparedSegments = append(preparedSegments, segment)
	}

	// Se qualquer trecho falhar no PREPARE: aborta imediatamente todos os que aceitaram
	if prepareError != nil {
		log.Printf("[coordinator] falha no PREPARE da tx_id=%s: %v. Abortando trechos preparados...", txID, prepareError)
		c.abortPreparedSegments(txID, preparedSegments)

		if err := c.wal.Append(WALEntry{
			TxID:          txID,
			Role:          "coordinator",
			Status:        TxAborted,
			CoordinatorID: c.companyID,
			SegmentIDs:    segmentIDStrings,
			Segments:      walSegments,
			UpdatedAt:     time.Now(),
		}); err != nil {
			log.Printf("[coordinator] erro ao persistir ABORTED no WAL tx_id=%s: %v", txID, err)
		}

		return prepareError
	}

	// DECISÃO DE COMMIT (Ponto de Não-Retorno)
	// O coordenador grava a decisão COMMIT de forma persistente no WAL com metadados completos
	// antes de enviar qualquer mensagem de confirmação para os participantes.
	if err := c.wal.Append(WALEntry{
		TxID:          txID,
		Role:          "coordinator",
		Status:        TxCommitted,
		CoordinatorID: c.companyID,
		SegmentIDs:    segmentIDStrings,
		Segments:      walSegments,
		UpdatedAt:     time.Now(),
	}); err != nil {
		// Falha de disco catastrófica antes de decidir commit: aborta
		c.abortPreparedSegments(txID, preparedSegments)
		return fmt.Errorf("falha ao persistir decisão de commit no WAL: %w", err)
	}

	log.Printf("[coordinator] decisão COMMIT confirmada no WAL para tx_id=%s. Efetivando assentos...", txID)

	// FASE 2: COMMIT
	var commitErr error
	for _, segment := range segments {
		if err := c.commitSegment(txID, segment); err != nil {
			commitErr = err
			break
		}
	}

	if commitErr != nil {
		log.Printf("[coordinator] falha ao comitar itinerário via 2PC tx_id=%s: %v", txID, commitErr)
		return commitErr
	}

	log.Printf("[coordinator] itinerário confirmado com sucesso via 2PC tx_id=%s", txID)
	return nil
}

func (c *Coordinator) prepareSegment(txID string, lamport uint64, passengerID uuid.UUID, segment models.ReservedSegment) prepareResult {
	if segment.CompanyID == c.companyID {
		// Trecho sob responsabilidade desta própria empresa
		ride, err := c.repo.GetRideByID(segment.RideID)
		if err != nil || ride.Cancelled {
			return prepareResult{segment: segment, ok: false, reason: "carona local indisponível"}
		}

		available := 0
		found := false
		for _, st := range ride.Segments {
			if st.ID == segment.SegmentID {
				available = st.AvailableSeats
				found = true
				break
			}
		}

		if !found || available <= 0 {
			return prepareResult{segment: segment, ok: false, reason: "sem assentos disponíveis no trecho local"}
		}

		lock := SegmentLock{
			TxID:          txID,
			CoordinatorID: c.companyID,
			LamportClock:  lamport,
		}

		acquired, _, err := c.lockStore.TryAcquire(segment.SegmentID.String(), lock)
		if err != nil || !acquired {
			return prepareResult{segment: segment, ok: false, reason: "conflito de lock concorrente no trecho local"}
		}

		// Marca lock como preparado localmente
		c.lockStore.MarkPrepared(segment.SegmentID.String(), txID)

		return prepareResult{segment: segment, ok: true}
	}

	// Trecho sob responsabilidade de um peer remoto
	peer, exists := c.peers.Get(segment.CompanyID)
	if !exists {
		return prepareResult{segment: segment, ok: false, reason: "empresa parceira não registrada no consórcio"}
	}

	reqBody, _ := json.Marshal(map[string]any{
		"tx_id":                  txID,
		"coordinator_company_id": c.companyID,
		"lamport_clock":          lamport,
		"passenger_id":           passengerID,
		"ride_id":                segment.RideID,
	})

	url := fmt.Sprintf("%s/internal/v1/segments/%s/prepare", peer.InternalURL, segment.SegmentID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return prepareResult{segment: segment, ok: false, reason: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[coordinator] falha de comunicação com empresa %s no trecho %s: %v", segment.CompanyID, segment.SegmentID, err)
		errStr := err.Error()
		if strings.Contains(errStr, "Client.Timeout") || strings.Contains(errStr, "context deadline exceeded") {
			return prepareResult{segment: segment, ok: false, reason: "servidor da empresa não respondeu a tempo (timeout)"}
		}
		if strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "connectex") {
			return prepareResult{segment: segment, ok: false, reason: "servidor da empresa está fora do ar (conexão recusada)"}
		}
		return prepareResult{segment: segment, ok: false, reason: "servidor da empresa temporariamente inacessível"}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errData struct {
			Reason string `json:"reason"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errData)
		reason := errData.Reason
		if reason == "" {
			reason = fmt.Sprintf("status HTTP %d", resp.StatusCode)
		}
		return prepareResult{segment: segment, ok: false, reason: reason}
	}

	var resData struct {
		OK           bool   `json:"ok"`
		LamportClock uint64 `json:"lamport_clock"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&resData); err != nil {
		return prepareResult{segment: segment, ok: false, reason: "resposta JSON inválida do peer"}
	}

	c.clock.Update(resData.LamportClock)
	return prepareResult{segment: segment, ok: resData.OK}
}

func (c *Coordinator) abortPreparedSegments(txID string, segments []models.ReservedSegment) {
	for _, segment := range segments {
		if segment.CompanyID == c.companyID {
			c.lockStore.Release(segment.SegmentID.String(), txID)
			continue
		}

		peer, exists := c.peers.Get(segment.CompanyID)
		if !exists {
			continue
		}

		reqBody, _ := json.Marshal(map[string]any{
			"tx_id":                  txID,
			"coordinator_company_id": c.companyID,
		})

		url := fmt.Sprintf("%s/internal/v1/segments/%s/abort", peer.InternalURL, segment.SegmentID)
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(reqBody))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			resp, err := c.httpClient.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}
	}
}

func (c *Coordinator) commitSegment(txID string, segment models.ReservedSegment) error {
	if segment.CompanyID == c.companyID {
		if err := c.repo.DecrementSeatIdempotent(segment.RideID, segment.SegmentID, txID); err != nil {
			c.lockStore.Release(segment.SegmentID.String(), txID)
			return err
		}
		c.lockStore.Release(segment.SegmentID.String(), txID)
		return nil
	}

	peer, exists := c.peers.Get(segment.CompanyID)
	if !exists {
		err := fmt.Errorf("peer %s não encontrado para tx_id=%s", segment.CompanyID, txID)
		log.Printf("[coordinator] %v", err)
		return err
	}

	reqBody, _ := json.Marshal(map[string]any{
		"tx_id":                  txID,
		"coordinator_company_id": c.companyID,
		"ride_id":                segment.RideID,
	})

	url := fmt.Sprintf("%s/internal/v1/segments/%s/commit", peer.InternalURL, segment.SegmentID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		log.Printf("[coordinator] erro ao montar commit para peer %s: %v", segment.CompanyID, err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[coordinator] falha de comunicação no commit para peer %s (será retransmitido no recovery): %v", segment.CompanyID, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errData struct {
			Reason string `json:"reason"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errData)
		reason := errData.Reason
		if reason == "" {
			reason = fmt.Sprintf("status HTTP %d", resp.StatusCode)
		}
		log.Printf("[coordinator] commit retornou erro do peer %s para tx_id=%s: %s", segment.CompanyID, txID, reason)
		return fmt.Errorf("peer %s rejeitou commit: %s", segment.CompanyID, reason)
	}

	return nil
}

// RecoverPendingCommits varre o WAL na inicialização do servidor e retransmite
// confirmações para quaisquer transações decididas com COMMITTED antes de uma eventual queda.
// Cada trecho é enviado exclusivamente à empresa proprietária acompanhado do respectivo ride_id,
// e trechos locais da empresa coordenadora são efetivados diretamente no repositório de forma idempotente.
func (c *Coordinator) RecoverPendingCommits() {
	entries, err := c.wal.ReadAll()
	if err != nil {
		log.Printf("[coordinator] erro ao ler WAL no recovery: %v", err)
		return
	}

	for _, entry := range entries {
		if entry.Role == "coordinator" && entry.Status == TxCommitted {
			log.Printf("[coordinator] recuperando transação COMMITTED pendente tx_id=%s", entry.TxID)

			if len(entry.Segments) > 0 {
				for _, seg := range entry.Segments {
					if seg.CompanyID == c.companyID {
						// 1. Trecho local do coordenador: efetivação no próprio repositório
						if err := c.repo.DecrementSeatIdempotent(seg.RideID, seg.SegmentID, entry.TxID); err != nil {
							log.Printf("[coordinator] recovery trecho local seg=%s tx_id=%s: %v", seg.SegmentID, entry.TxID, err)
						}
						c.lockStore.Release(seg.SegmentID.String(), entry.TxID)
					} else {
						// 2. Trecho remoto: encaminhamento exclusivo ao peer proprietário contendo o ride_id
						peer, exists := c.peers.Get(seg.CompanyID)
						if !exists {
							log.Printf("[coordinator] peer %s não encontrado para recovery seg=%s tx_id=%s", seg.CompanyID, seg.SegmentID, entry.TxID)
							continue
						}

						url := fmt.Sprintf("%s/internal/v1/segments/%s/commit", peer.InternalURL, seg.SegmentID)
						reqBody, _ := json.Marshal(map[string]any{
							"tx_id":                  entry.TxID,
							"coordinator_company_id": c.companyID,
							"ride_id":                seg.RideID,
						})
						req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
						if err == nil {
							req.Header.Set("Content-Type", "application/json")
							resp, err := c.httpClient.Do(req)
							if err == nil {
								resp.Body.Close()
							} else {
								log.Printf("[coordinator] recovery peer %s falhou na conexão: %v", seg.CompanyID, err)
							}
						}
					}
				}
			}
		}
	}
}
