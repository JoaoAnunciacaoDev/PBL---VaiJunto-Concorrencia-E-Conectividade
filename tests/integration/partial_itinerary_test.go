package integration

import (
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/google/uuid"
)

// TestNoPartialItinerary valida com injeção de falhas que sob nenhuma hipótese um
// itinerário com trechos de empresas distintas é confirmado pela metade (atomicidade total).
func TestNoPartialItinerary(t *testing.T) {
	cluster := NewTestCluster(t, 5*time.Second)

	now := time.Now().Add(24 * time.Hour).Truncate(time.Hour)

	// Trecho 1: Salvador -> Feira de Santana na Empresa A (2 assentos)
	rideA, err := CreateTestRide(
		cluster.Nodes["empresa-a"].Server.Repository(),
		"driver-a@vaijunto.test",
		enum.Salvador,
		enum.FeiraDeSantana,
		now,
		2,
		4500,
	)
	if err != nil {
		t.Fatalf("falha ao criar carona A: %v", err)
	}
	segA := rideA.Segments[0]

	// Trecho 2: Feira de Santana -> Alagoinhas na Empresa B (2 assentos)
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

	// -------------------------------------------------------------------------
	// Cenário 1: Injeção de Falha — Queda súbita da Empresa B durante a reserva
	// -------------------------------------------------------------------------
	t.Run("QuedaDoServidorParticipanteDuranteReserva", func(t *testing.T) {
		// Simula a queda física da empresa-b
		cluster.StopNode("empresa-b")

		// Conecta passageiro na empresa-a
		paxClient, err := RegisterAndLoginPassenger(
			cluster.Nodes["empresa-a"].Server.TCPAddr(),
			"Passageiro Falha",
			"pax-falha@test.com",
			"Senha@123",
		)
		if err != nil {
			t.Fatalf("falha ao autenticar passageiro: %v", err)
		}
		defer paxClient.Close()

		// Tenta reservar o itinerário composto [Trecho A (empresa-a) + Trecho B (empresa-b)]
		req := protocol.ConfirmReservationRequest{
			Segments: []models.ReservedSegment{
				{
					RideID:    rideA.ID,
					SegmentID: segA.ID,
					CompanyID: "empresa-a",
				},
				{
					RideID:    rideB.ID,
					SegmentID: segB.ID,
					CompanyID: "empresa-b",
				},
			},
		}

		resp, err := paxClient.Send(protocol.ActionConfirmReservation, req)
		if err != nil {
			t.Fatalf("erro inesperado na conexão TCP do passageiro: %v", err)
		}

		// A reserva DEVE ter sido recusada
		if resp.Success != "error" {
			t.Fatalf("reserva multitrecho deveria ter falhado por queda do participante, obtido: %+v", resp)
		}
		t.Logf("Resposta esperada de falha na reserva composta: message=%q", resp.Message)

		// Verifica que o Trecho A na Empresa A NÃO foi decrementado (rollback do 2PC)
		checkRideA, err := cluster.Nodes["empresa-a"].Server.Repository().GetRideByID(rideA.ID)
		if err != nil {
			t.Fatalf("erro ao consultar carona A: %v", err)
		}
		if checkRideA.Segments[0].AvailableSeats != 2 {
			t.Fatalf("ATOMICIDADE QUEBRADA: assentos do Trecho A foram decrementados (%d, esperado 2)",
				checkRideA.Segments[0].AvailableSeats)
		}

		// Verifica que o LockStore da Empresa A está limpo para esse trecho
		_, hasLock := cluster.Nodes["empresa-a"].Server.LockStore().Get(segA.ID.String())
		if hasLock {
			t.Fatal("ATOMICIDADE QUEBRADA: lock do Trecho A permaneceu retido após falha de 2PC")
		}

		// Verifica que nenhuma reserva foi criada na Empresa A
		paxReservations, err := cluster.Nodes["empresa-a"].Server.Repository().GetReservationsByPassengerID(uuid.Nil)
		if err == nil && len(paxReservations) > 0 {
			t.Fatalf("reserva fantasma encontrada na empresa A: %+v", paxReservations)
		}
	})

	// -------------------------------------------------------------------------
	// Cenário 2: Injeção de Falha — Participante sem assento (rejeição no PREPARE)
	// -------------------------------------------------------------------------
	t.Run("RejeicaoDeTrechoPorSemVagaEmParticipante", func(t *testing.T) {
		// Reinicia a empresa-b
		_ = cluster.RestartNode("empresa-b")

		// Cria carona esgotada (0 assentos) na empresa-b
		rideBFull, err := CreateTestRide(
			cluster.Nodes["empresa-b"].Server.Repository(),
			"driver-b2@vaijunto.test",
			enum.FeiraDeSantana,
			enum.Alagoinhas,
			now.Add(3*time.Hour),
			0, // 0 vagas
			3500,
		)
		if err != nil {
			t.Fatalf("falha ao criar carona esgotada: %v", err)
		}

		paxClient, err := RegisterAndLoginPassenger(
			cluster.Nodes["empresa-a"].Server.TCPAddr(),
			"Passageiro Sem Vaga",
			"pax-semvaga@test.com",
			"Senha@123",
		)
		if err != nil {
			t.Fatalf("falha ao autenticar passageiro: %v", err)
		}
		defer paxClient.Close()

		// Tenta reservar Trecho A (empresa-a com 2 vagas) + Trecho B2 (empresa-b com 0 vagas)
		req := protocol.ConfirmReservationRequest{
			Segments: []models.ReservedSegment{
				{
					RideID:    rideA.ID,
					SegmentID: segA.ID,
					CompanyID: "empresa-a",
				},
				{
					RideID:    rideBFull.ID,
					SegmentID: rideBFull.Segments[0].ID,
					CompanyID: "empresa-b",
				},
			},
		}

		resp, err := paxClient.Send(protocol.ActionConfirmReservation, req)
		if err != nil {
			t.Fatalf("erro de conexão: %v", err)
		}

		if resp.Success != "error" {
			t.Fatalf("reserva deveria ter falhado por falta de vaga em B, obtido: %+v", resp)
		}

		// Trecho A na empresa-a deve permanecer com seus 2 assentos
		checkRideA, _ := cluster.Nodes["empresa-a"].Server.Repository().GetRideByID(rideA.ID)
		if checkRideA.Segments[0].AvailableSeats != 2 {
			t.Fatalf("Trecho A foi indevidamente decrementado para %d", checkRideA.Segments[0].AvailableSeats)
		}
	})
}
