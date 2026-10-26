package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/consortium"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/httpapi"
	"github.com/google/uuid"
)

// TestCoordinatorCrashAfterCommitted valida que, quando o coordenador persiste a decisão
// de COMMITTED no WAL mas cai antes de disparar a Fase 2 (commit) para os participantes,
// ao reiniciar o mecanismo RecoverPendingCommits:
// 1. Efetiva os trechos locais no próprio repositório de forma idempotente.
// 2. Encaminha a confirmação de commit com ride_id exclusivamente para o peer proprietário.
// 3. Garante que os assentos em ambos os nós refletem exatamente a reserva sem duplicação.
func TestCoordinatorCrashAfterCommitted(t *testing.T) {
	cluster := NewTestCluster(t, 5*time.Second)

	now := time.Now().Add(24 * time.Hour).Truncate(time.Hour)

	// Trecho 1: Local na Empresa A (2 assentos)
	rideA, err := CreateTestRide(
		cluster.Nodes["empresa-a"].Server.Repository(),
		"driver-a@vaijunto.test",
		enum.Salvador,
		enum.FeiraDeSantana,
		now,
		2,
		4000,
	)
	if err != nil {
		t.Fatalf("falha ao criar carona A: %v", err)
	}
	segA := rideA.Segments[0]

	// Trecho 2: Remoto na Empresa B (2 assentos)
	rideB, err := CreateTestRide(
		cluster.Nodes["empresa-b"].Server.Repository(),
		"driver-b@vaijunto.test",
		enum.FeiraDeSantana,
		enum.Alagoinhas,
		now.Add(3*time.Hour),
		2,
		3500,
	)
	if err != nil {
		t.Fatalf("falha ao criar carona B: %v", err)
	}
	segB := rideB.Segments[0]

	txID := uuid.New().String()
	passengerID := uuid.New()

	// 1. Envia PREPARE para o trecho local na Empresa A
	lockA := consortium.SegmentLock{
		TxID:          txID,
		CoordinatorID: "empresa-a",
		LamportClock:  1,
	}
	acquired, _, err := cluster.Nodes["empresa-a"].Server.LockStore().TryAcquire(segA.ID.String(), lockA)
	if err != nil || !acquired {
		t.Fatalf("falha ao adquirir lock local em A: %v", err)
	}
	cluster.Nodes["empresa-a"].Server.LockStore().MarkPrepared(segA.ID.String(), txID)

	// 2. Envia PREPARE via HTTP para o trecho remoto na Empresa B
	httpClient := &http.Client{Timeout: 2 * time.Second}
	prepareURL := fmt.Sprintf("http://%s/internal/v1/segments/%s/prepare",
		cluster.Nodes["empresa-b"].Server.HTTPAddr(), segB.ID.String())

	prepBody, _ := json.Marshal(httpapi.PrepareRequest{
		TxID:                 txID,
		CoordinatorCompanyID: "empresa-a",
		LamportClock:         2,
		PassengerID:          passengerID,
		RideID:               rideB.ID,
	})

	prepResp, err := httpClient.Post(prepareURL, "application/json", bytes.NewReader(prepBody))
	if err != nil || prepResp.StatusCode != http.StatusOK {
		t.Fatalf("prepare falhou na empresa B: err=%v resp=%+v", err, prepResp)
	}
	_ = prepResp.Body.Close()

	// 3. Simula a decisão COMMITTED sendo gravada pelo Coordenador (Empresa A) no WAL
	// com os metadados completos de ambos os trechos (local e remoto)
	walPath := cluster.Nodes["empresa-a"].Server.Repository()
	_ = walPath
	walEntry := consortium.WALEntry{
		TxID:          txID,
		Role:          "coordinator",
		Status:        consortium.TxCommitted,
		CoordinatorID: "empresa-a",
		SegmentIDs:    []string{segA.ID.String(), segB.ID.String()},
		Segments: []consortium.WALSegment{
			{
				SegmentID: segA.ID,
				RideID:    rideA.ID,
				CompanyID: "empresa-a",
			},
			{
				SegmentID: segB.ID,
				RideID:    rideB.ID,
				CompanyID: "empresa-b",
			},
		},
		UpdatedAt: time.Now(),
	}

	// Simula a queda do Coordenador ANTES de enviar o commit para qualquer um dos nós!
	t.Log("Gravando COMMITTED no WAL da empresa A e simulando queda imediata do coordenador...")
	// Gravamos no WAL da empresa-a antes de parar o nó
	walA, err := consortium.NewWAL(cluster.Nodes["empresa-a"].DataDir + "/wal.log")
	if err != nil {
		t.Fatalf("erro ao reabrir WAL de A: %v", err)
	}
	if err := walA.Append(walEntry); err != nil {
		t.Fatalf("erro ao gravar WAL: %v", err)
	}
	_ = walA.Close()

	cluster.StopNode("empresa-a")

	// 4. Verifica que antes da recuperação os assentos ainda NÃO foram debitados
	checkRideB, _ := cluster.Nodes["empresa-b"].Server.Repository().GetRideByID(rideB.ID)
	if checkRideB.Segments[0].AvailableSeats != 2 {
		t.Fatalf("empresa B não deveria ter decrementado antes do commit: %d", checkRideB.Segments[0].AvailableSeats)
	}

	// 5. Reinicia o nó Coordenador (Empresa A)
	t.Log("Reiniciando Coordenador Empresa A e disparando RecoverPendingCommits...")
	restartedNodeA := cluster.RestartNode("empresa-a")

	// Aguarda processamento de recuperação
	time.Sleep(500 * time.Millisecond)

	// 6. Verifica se o trecho local da Empresa A foi decrementado corretamente (de 2 para 1)
	updatedRideA, err := restartedNodeA.Server.Repository().GetRideByID(rideA.ID)
	if err != nil {
		t.Fatalf("erro ao consultar carona A após recovery: %v", err)
	}
	if updatedRideA.Segments[0].AvailableSeats != 1 {
		t.Fatalf("assento local de A deveria ter sido decrementado para 1, obtido: %d",
			updatedRideA.Segments[0].AvailableSeats)
	}

	// 7. Verifica se o trecho remoto da Empresa B foi decrementado corretamente (de 2 para 1)
	updatedRideB, err := cluster.Nodes["empresa-b"].Server.Repository().GetRideByID(rideB.ID)
	if err != nil {
		t.Fatalf("erro ao consultar carona B após recovery: %v", err)
	}
	if updatedRideB.Segments[0].AvailableSeats != 1 {
		t.Fatalf("assento remoto de B deveria ter sido decrementado para 1 pelo recovery, obtido: %d",
			updatedRideB.Segments[0].AvailableSeats)
	}

	// 8. Executa uma segunda rodada de recuperação no Coordenador para provar IDEMPOTÊNCIA
	t.Log("Disparando segunda rodada de recovery para validar idempotência...")
	restartedNodeA.Server.Coordinator().RecoverPendingCommits()
	time.Sleep(200 * time.Millisecond)

	// Os assentos devem continuar exatamente em 1 (sem duplo decremento)
	finalRideA, _ := restartedNodeA.Server.Repository().GetRideByID(rideA.ID)
	if finalRideA.Segments[0].AvailableSeats != 1 {
		t.Fatalf("DUPLO DECREMENTO EM A: assentos=%d, esperado 1", finalRideA.Segments[0].AvailableSeats)
	}

	finalRideB, _ := cluster.Nodes["empresa-b"].Server.Repository().GetRideByID(rideB.ID)
	if finalRideB.Segments[0].AvailableSeats != 1 {
		t.Fatalf("DUPLO DECREMENTO EM B: assentos=%d, esperado 1", finalRideB.Segments[0].AvailableSeats)
	}
}

// TestCommitIdempotencyOnRetries valida que chamadas repetidas ou duplicadas
// de POST /commit para o mesmo tx_id decrementam o assento estritamente uma única vez.
func TestCommitIdempotencyOnRetries(t *testing.T) {
	cluster := NewTestCluster(t, 5*time.Second)

	now := time.Now().Add(24 * time.Hour).Truncate(time.Hour)
	ride, err := CreateTestRide(
		cluster.Nodes["empresa-b"].Server.Repository(),
		"driver-b@vaijunto.test",
		enum.Salvador,
		enum.FeiraDeSantana,
		now,
		3, // 3 vagas
		5000,
	)
	if err != nil {
		t.Fatalf("criar carona: %v", err)
	}
	seg := ride.Segments[0]

	httpClient := &http.Client{Timeout: 2 * time.Second}
	txID := uuid.New().String()

	// 1. Prepare
	prepURL := fmt.Sprintf("http://%s/internal/v1/segments/%s/prepare",
		cluster.Nodes["empresa-b"].Server.HTTPAddr(), seg.ID.String())
	prepBody, _ := json.Marshal(httpapi.PrepareRequest{
		TxID:                 txID,
		CoordinatorCompanyID: "empresa-a",
		LamportClock:         10,
		PassengerID:          uuid.New(),
		RideID:               ride.ID,
	})
	prepResp, err := httpClient.Post(prepURL, "application/json", bytes.NewReader(prepBody))
	if err != nil || prepResp.StatusCode != http.StatusOK {
		t.Fatalf("prepare falhou: %v", err)
	}
	_ = prepResp.Body.Close()

	// 2. Dispara 3 chamadas idênticas de commit consecutivas para o mesmo txID
	commitURL := fmt.Sprintf("http://%s/internal/v1/segments/%s/commit",
		cluster.Nodes["empresa-b"].Server.HTTPAddr(), seg.ID.String())
	commitBody, _ := json.Marshal(httpapi.CommitRequest{
		TxID:                 txID,
		CoordinatorCompanyID: "empresa-a",
		RideID:               ride.ID,
	})

	for attempt := 1; attempt <= 3; attempt++ {
		resp, err := httpClient.Post(commitURL, "application/json", bytes.NewReader(commitBody))
		if err != nil {
			t.Fatalf("tentativa %d falhou na rede: %v", attempt, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("tentativa %d retornou status %d", attempt, resp.StatusCode)
		}
		_ = resp.Body.Close()
	}

	// 3. Verifica que o assento caiu de 3 para exatamente 2 (1 único decremento)
	finalRide, _ := cluster.Nodes["empresa-b"].Server.Repository().GetRideByID(ride.ID)
	if finalRide.Segments[0].AvailableSeats != 2 {
		t.Fatalf("esperado 2 assentos restantes, obtido %d", finalRide.Segments[0].AvailableSeats)
	}
}

// TestLateAbortIgnoredAfterCommit valida que, após uma transação ter sido comitada,
// qualquer mensagem de ABORT atrasada recebida posteriormente não reverte o assento
// nem corrompe o estado final COMMITTED no WAL.
func TestLateAbortIgnoredAfterCommit(t *testing.T) {
	cluster := NewTestCluster(t, 5*time.Second)

	now := time.Now().Add(24 * time.Hour).Truncate(time.Hour)
	ride, err := CreateTestRide(
		cluster.Nodes["empresa-c"].Server.Repository(),
		"driver-c@vaijunto.test",
		enum.Salvador,
		enum.FeiraDeSantana,
		now,
		2,
		4500,
	)
	if err != nil {
		t.Fatalf("criar carona: %v", err)
	}
	seg := ride.Segments[0]

	httpClient := &http.Client{Timeout: 2 * time.Second}
	txID := uuid.New().String()

	// 1. Prepare
	prepURL := fmt.Sprintf("http://%s/internal/v1/segments/%s/prepare",
		cluster.Nodes["empresa-c"].Server.HTTPAddr(), seg.ID.String())
	prepBody, _ := json.Marshal(httpapi.PrepareRequest{
		TxID:                 txID,
		CoordinatorCompanyID: "empresa-a",
		LamportClock:         10,
		PassengerID:          uuid.New(),
		RideID:               ride.ID,
	})
	resp, err := httpClient.Post(prepURL, "application/json", bytes.NewReader(prepBody))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("prepare falhou: %v", err)
	}
	_ = resp.Body.Close()

	// 2. Commit
	commitURL := fmt.Sprintf("http://%s/internal/v1/segments/%s/commit",
		cluster.Nodes["empresa-c"].Server.HTTPAddr(), seg.ID.String())
	commitBody, _ := json.Marshal(httpapi.CommitRequest{
		TxID:                 txID,
		CoordinatorCompanyID: "empresa-a",
		RideID:               ride.ID,
	})
	resp, err = httpClient.Post(commitURL, "application/json", bytes.NewReader(commitBody))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("commit falhou: %v", err)
	}
	_ = resp.Body.Close()

	// Verifica assento = 1
	checkRide, _ := cluster.Nodes["empresa-c"].Server.Repository().GetRideByID(ride.ID)
	if checkRide.Segments[0].AvailableSeats != 1 {
		t.Fatalf("assento deveria ser 1 após commit, obtido %d", checkRide.Segments[0].AvailableSeats)
	}

	// 3. Envia ABORT atrasado para a mesma transação
	abortURL := fmt.Sprintf("http://%s/internal/v1/segments/%s/abort",
		cluster.Nodes["empresa-c"].Server.HTTPAddr(), seg.ID.String())
	abortBody, _ := json.Marshal(httpapi.AbortRequest{
		TxID:                 txID,
		CoordinatorCompanyID: "empresa-a",
	})
	resp, err = httpClient.Post(abortURL, "application/json", bytes.NewReader(abortBody))
	if err != nil {
		t.Fatalf("abort atrasado falhou na conexão: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperado status 200 no abort tardio (no-op), obtido %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// 4. O assento DEVE permanecer em 1 (não foi devolvido indevidamente)
	afterAbortRide, _ := cluster.Nodes["empresa-c"].Server.Repository().GetRideByID(ride.ID)
	if afterAbortRide.Segments[0].AvailableSeats != 1 {
		t.Fatalf("REGRESSÃO DE ESTADO: assento foi revertido indevidamente após abort tardio (%d)",
			afterAbortRide.Segments[0].AvailableSeats)
	}

	// 5. O estado no endpoint de status deve continuar COMMITTED
	statusURL := fmt.Sprintf("http://%s/internal/v1/segments/%s/status/%s",
		cluster.Nodes["empresa-c"].Server.HTTPAddr(), seg.ID.String(), txID)
	resp, err = httpClient.Get(statusURL)
	if err != nil {
		t.Fatalf("consulta de status: %v", err)
	}
	var stat httpapi.StatusResponse
	_ = json.NewDecoder(resp.Body).Decode(&stat)
	_ = resp.Body.Close()

	if stat.Status != string(consortium.TxCommitted) {
		t.Fatalf("status no WAL regrediu para %s, esperado COMMITTED", stat.Status)
	}
}

// TestPreparedLockImmuneToEvictionAndTTL valida que um lock preparado não pode ser
// desalojado por transações concorrentes nem ser descartado passivamente pelo reaper de TTL.
func TestPreparedLockImmuneToEvictionAndTTL(t *testing.T) {
	// Cria store com TTL curto de 1 segundo
	lockStore := consortium.NewLockStore(1 * time.Second)
	segID := uuid.New().String()

	tx1 := "tx-1-prepared"
	lock1 := consortium.SegmentLock{
		TxID:          tx1,
		CoordinatorID: "empresa-b",
		LamportClock:  100, // relógio maior (pior prioridade lógica)
	}

	acquired, _, err := lockStore.TryAcquire(segID, lock1)
	if err != nil || !acquired {
		t.Fatalf("falha ao adquirir lock 1: %v", err)
	}

	// Marca tx1 como PREPARADA
	lockStore.MarkPrepared(segID, tx1)

	// 1. Tenta adquirir com tx2 que possui LamportClock menor (10) e menor CoordinatorID (empresa-a)
	// Em circunstâncias normais tx2 desalojaria tx1, mas como tx1 está PREPARED, tx2 DEVE ser recusada!
	tx2 := "tx-2-concorrente"
	lock2 := consortium.SegmentLock{
		TxID:          tx2,
		CoordinatorID: "empresa-a",
		LamportClock:  10, // prioridade estritamente melhor
	}

	acquired2, evicted, err := lockStore.TryAcquire(segID, lock2)
	if err != nil {
		t.Fatalf("erro em TryAcquire: %v", err)
	}
	if acquired2 || evicted != "" {
		t.Fatalf("FALHA DE CONSISTÊNCIA: transação preparada foi desalojada! acquired=%v evicted=%s", acquired2, evicted)
	}

	// 2. Aguarda o tempo de expiração do TTL (1.5 segundos)
	time.Sleep(1500 * time.Millisecond)

	// Executa varredura de expiração
	expired := lockStore.ExpireAll()
	if len(expired) != 0 {
		t.Fatalf("FALHA: lock preparado foi expirado pelo reaper! expired=%+v", expired)
	}

	// O lock ainda deve pertencer a tx1
	currentLock, exists := lockStore.Get(segID)
	if !exists || currentLock.TxID != tx1 || !currentLock.IsPrepared {
		t.Fatalf("lock preparado desapareceu ou foi alterado: exists=%v lock=%+v", exists, currentLock)
	}
}
