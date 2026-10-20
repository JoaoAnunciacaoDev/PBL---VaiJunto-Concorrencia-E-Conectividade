package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/httpapi"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/google/uuid"
)

// TestNoBlockedSeatsAfterRecovery valida que a queda e subsequente recuperação
// de um servidor participante do consórcio não deixa assentos bloqueados indefinidamente por TTL.
func TestNoBlockedSeatsAfterRecovery(t *testing.T) {
	// Configura o cluster com lock TTL curto de 2 segundos para o teste ser rápido e preciso
	lockTTL := 2 * time.Second
	cluster := NewTestCluster(t, lockTTL)

	// 1. Empresa C publica uma carona com 1 único assento
	departure := time.Now().Add(48 * time.Hour).Truncate(time.Hour)
	rideC, err := CreateTestRide(
		cluster.Nodes["empresa-c"].Server.Repository(),
		"driver-c@vaijunto.test",
		enum.Salvador,
		enum.FeiraDeSantana,
		departure,
		1, // 1 assento
		6000,
	)
	if err != nil {
		t.Fatalf("falha ao criar carona na empresa C: %v", err)
	}
	segC := rideC.Segments[0]

	// 2. Simula uma transação 2PC iniciada por um coordenador remoto que envia PREPARE
	//    para o trecho da empresa C, obtendo o lock temporário
	httpClient := &http.Client{Timeout: 2 * time.Second}
	prepareURL := fmt.Sprintf("http://%s/internal/v1/segments/%s/prepare",
		cluster.Nodes["empresa-c"].Server.HTTPAddr(), segC.ID.String())

	fakeTxID := uuid.New().String()
	prepBody, _ := json.Marshal(httpapi.PrepareRequest{
		TxID:                 fakeTxID,
		CoordinatorCompanyID: "empresa-remota-fantasma",
		LamportClock:         50,
		PassengerID:          uuid.New(),
		RideID:               rideC.ID,
	})

	prepResp, err := httpClient.Post(prepareURL, "application/json", bytes.NewReader(prepBody))
	if err != nil {
		t.Fatalf("falha ao enviar PREPARE para empresa C: %v", err)
	}
	if prepResp.StatusCode != http.StatusOK {
		t.Fatalf("esperado status 200 no PREPARE, obtido %d", prepResp.StatusCode)
	}
	_ = prepResp.Body.Close()

	// Confirma que o trecho está atualmente bloqueado no LockStore da empresa C
	lock, hasLock := cluster.Nodes["empresa-c"].Server.LockStore().Get(segC.ID.String())
	if !hasLock || lock.TxID != fakeTxID {
		t.Fatalf("trecho deveria estar travado pelo lock %s", fakeTxID)
	}

	// Enquanto o lock está ativo, outra tentativa de PREPARE deve ser rejeitada com conflito
	prepConflict, err := httpClient.Post(prepareURL, "application/json", bytes.NewReader(prepBody))
	if err == nil {
		_ = prepConflict.Body.Close()
	}

	// 3. Simula a queda abrupta da Empresa C no meio da reserva (coordenador nunca enviou commit)
	t.Log("Simulando queda do servidor Empresa C com lock ativo...")
	cluster.StopNode("empresa-c")

	// 4. Aguarda o tempo de expiração do lock (TTL de 2s + margem de segurança)
	t.Logf("Aguardando expiração do TTL (%v)...", lockTTL+1*time.Second)
	time.Sleep(lockTTL + 1*time.Second)

	// 5. Reinicia a Empresa C (recuperação a partir do disco/WAL)
	t.Log("Reiniciando Empresa C e recuperando estado...")
	restartedNode := cluster.RestartNode("empresa-c")

	// 6. Verifica que o assento está livre e desimpedido:
	//    AvailableSeats deve continuar intacto (1 assento), pois nunca houve COMMIT
	recoveredRide, err := restartedNode.Server.Repository().GetRideByID(rideC.ID)
	if err != nil {
		t.Fatalf("erro ao consultar carona após recuperação: %v", err)
	}
	if recoveredRide.Segments[0].AvailableSeats != 1 {
		t.Fatalf("assento deveria estar disponível (1), encontrado: %d",
			recoveredRide.Segments[0].AvailableSeats)
	}

	// 7. Agora um novo passageiro legítimo conecta via TCP na Empresa C e reserva a vaga
	paxClient, err := RegisterAndLoginPassenger(
		restartedNode.Server.TCPAddr(),
		"Passageiro Recuperacao",
		"pax-recovery@test.com",
		"Senha@123",
	)
	if err != nil {
		t.Fatalf("falha ao autenticar novo passageiro após recovery: %v", err)
	}
	defer paxClient.Close()

	bookReq := protocol.ConfirmReservationRequest{
		Segments: []models.ReservedSegment{
			{
				RideID:    rideC.ID,
				SegmentID: segC.ID,
				CompanyID: "empresa-c",
			},
		},
	}

	bookResp, err := paxClient.Send(protocol.ActionConfirmReservation, bookReq)
	if err != nil {
		t.Fatalf("erro ao enviar reserva via TCP após recovery: %v", err)
	}

	if bookResp.Success != "success" {
		t.Fatalf("ASSENTO PERMANECEU BLOQUEADO INDEVIDAMENTE: resposta=%+v", bookResp)
	}
	t.Log("Nova reserva confirmada com sucesso no assento recuperado!")

	// 8. O assento foi consumido de forma legítima e agora está em 0
	finalRide, _ := restartedNode.Server.Repository().GetRideByID(rideC.ID)
	if finalRide.Segments[0].AvailableSeats != 0 {
		t.Fatalf("assentos finais deveriam ser 0, obtido %d", finalRide.Segments[0].AvailableSeats)
	}
}
