package integration

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
)

// TestNoDoubleSeatBooking valida sob alta concorrência concorrente entre múltiplos servidores
// do consórcio que nenhum assento é confirmado para mais de um passageiro simultâneo.
func TestNoDoubleSeatBooking(t *testing.T) {
	cluster := NewTestCluster(t, 5*time.Second)

	// 1. Empresa A publica uma carona com exatamente 1 ÚNICO assento
	departure := time.Now().Add(24 * time.Hour).Truncate(time.Hour)
	ride, err := CreateTestRide(
		cluster.Nodes["empresa-a"].Server.Repository(),
		"driver-a@vaijunto.test",
		enum.Salvador,
		enum.FeiraDeSantana,
		departure,
		1, // apenas 1 assento disponível
		5000,
	)
	if err != nil {
		t.Fatalf("falha ao criar carona na empresa-a: %v", err)
	}
	targetSegment := ride.Segments[0]

	// 2. Prepara 21 clientes distribuídos igualmente entre as 3 empresas:
	//    7 clientes na empresa-a (concorrem localmente)
	//    7 clientes na empresa-b (concorrem via 2PC inter-empresa)
	//    7 clientes na empresa-c (concorrem via 2PC inter-empresa)
	type clientTarget struct {
		companyID string
		tcpAddr   string
		client    *TestTCPClient
		email     string
	}

	targets := make([]clientTarget, 0, 21)
	companyOrder := []string{"empresa-a", "empresa-b", "empresa-c"}

	for i := 0; i < 21; i++ {
		compID := companyOrder[i%3]
		tcpAddr := cluster.Nodes[compID].Server.TCPAddr()
		email := fmt.Sprintf("pax-%s-%d@test.com", compID, i)
		client, err := RegisterAndLoginPassenger(tcpAddr, fmt.Sprintf("Passageiro %d", i), email, "Senha@123")
		if err != nil {
			t.Fatalf("falha ao autenticar cliente %d na %s: %v", i, compID, err)
		}
		defer client.Close()

		targets = append(targets, clientTarget{
			companyID: compID,
			tcpAddr:   tcpAddr,
			client:    client,
			email:     email,
		})
	}

	// 3. Dispara todas as 21 tentativas de reserva de forma estritamente simultânea
	var (
		startGate = make(chan struct{})
		wg        sync.WaitGroup
		mu        sync.Mutex
		successes = 0
		failures  = 0
		winner    string
	)

	for _, target := range targets {
		wg.Add(1)
		go func(tg clientTarget) {
			defer wg.Done()

			<-startGate // aguarda barreira de sincronização

			req := protocol.ConfirmReservationRequest{
				Segments: []models.ReservedSegment{
					{
						RideID:    ride.ID,
						SegmentID: targetSegment.ID,
						CompanyID: "empresa-a",
					},
				},
			}

			resp, err := tg.client.Send(protocol.ActionConfirmReservation, req)
			mu.Lock()
			defer mu.Unlock()

			if err == nil && resp.Success == "success" {
				successes++
				winner = fmt.Sprintf("%s (%s)", tg.email, tg.companyID)
			} else {
				failures++
			}
		}(target)
	}

	// Libera a largada simultânea
	close(startGate)
	wg.Wait()

	// 4. Verificações de corretude distribuída
	t.Logf("Resultado da disputa: %d confirmada(s) (vencedor: %s), %d rejeitada(s)", successes, winner, failures)

	if successes != 1 {
		t.Fatalf("CORRUPÇÃO DE DADOS: esperado exatamente 1 reserva confirmada, obtido %d", successes)
	}

	if failures != 20 {
		t.Fatalf("esperado exatamente 20 rejeições de concorrência, obtido %d", failures)
	}

	// Consulta o estado no repositório dono do assento (empresa-a)
	confirmedPassengers, err := cluster.Nodes["empresa-a"].Server.Repository().GetConfirmedPassengerIDsByRideID(ride.ID)
	if err != nil {
		t.Fatalf("erro ao consultar passageiros confirmados: %v", err)
	}

	passengersOnSeg := confirmedPassengers[targetSegment.ID]
	if len(passengersOnSeg) != 1 {
		t.Fatalf("CORRUPÇÃO: trecho possui %d passageiros confirmados, esperado exatamente 1", len(passengersOnSeg))
	}

	// Verifica que o assento restante é 0
	updatedRide, err := cluster.Nodes["empresa-a"].Server.Repository().GetRideByID(ride.ID)
	if err != nil {
		t.Fatalf("erro ao consultar carona: %v", err)
	}

	if updatedRide.Segments[0].AvailableSeats != 0 {
		t.Fatalf("assentos disponíveis = %d, esperado exatamente 0", updatedRide.Segments[0].AvailableSeats)
	}
}
