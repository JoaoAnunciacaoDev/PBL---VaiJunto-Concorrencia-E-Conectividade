package integration

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
)

func TestDistributedCancellationAndDuplicateCheck(t *testing.T) {
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

	// Trecho 2: Feira de Santana -> Lençóis na Empresa B (2 assentos)
	rideB, err := CreateTestRide(
		cluster.Nodes["empresa-b"].Server.Repository(),
		"driver-b@vaijunto.test",
		enum.FeiraDeSantana,
		enum.Lencois,
		now.Add(3*time.Hour),
		2,
		6500,
	)
	if err != nil {
		t.Fatalf("falha ao criar carona B: %v", err)
	}
	segB := rideB.Segments[0]

	paxClient, err := RegisterAndLoginPassenger(
		cluster.Nodes["empresa-a"].Server.TCPAddr(),
		"Passageiro Distribuído",
		"pax-distrib@vaijunto.test",
		"Senha@123",
	)
	if err != nil {
		t.Fatalf("falha ao registrar/logar passageiro: %v", err)
	}
	defer paxClient.Close()

	// 1. Confirma a reserva multi-empresa (Empresa A + Empresa B)
	bookReq := protocol.ConfirmReservationRequest{
		Segments: []models.ReservedSegment{
			{RideID: rideA.ID, SegmentID: segA.ID, CompanyID: "empresa-a"},
			{RideID: rideB.ID, SegmentID: segB.ID, CompanyID: "empresa-b"},
		},
	}

	bookResp, err := paxClient.Send(protocol.ActionConfirmReservation, bookReq)
	if err != nil {
		t.Fatalf("falha ao enviar confirm_reservation: %v", err)
	}
	if bookResp.Success != "success" {
		t.Fatalf("reserva multi-empresa falhou: %s", bookResp.Message)
	}

	var res protocol.ReservationResponse
	if err := json.Unmarshal(bookResp.Payload, &res); err != nil {
		t.Fatalf("falha ao deserializar resposta da reserva: %v", err)
	}

	// Valida que os assentos diminuíram em ambos os nós
	rAAfter, _ := cluster.Nodes["empresa-a"].Server.Repository().GetRideByID(rideA.ID)
	rBAfter, _ := cluster.Nodes["empresa-b"].Server.Repository().GetRideByID(rideB.ID)
	if rAAfter.Segments[0].AvailableSeats != 1 {
		t.Fatalf("esperava 1 assento na empresa-a, obteve %d", rAAfter.Segments[0].AvailableSeats)
	}
	if rBAfter.Segments[0].AvailableSeats != 1 {
		t.Fatalf("esperava 1 assento na empresa-b, obteve %d", rBAfter.Segments[0].AvailableSeats)
	}

	// 2. Teste de prevenção de reserva duplicada para o mesmo passageiro
	dupResp, err := paxClient.Send(protocol.ActionConfirmReservation, bookReq)
	if err != nil {
		t.Fatalf("falha ao enviar reserva duplicada: %v", err)
	}
	if dupResp.Success != "error" || dupResp.Message != "Você já possui uma reserva confirmada para este itinerário." {
		t.Fatalf("esperava erro de reserva duplicada com mensagem amigável, obteve: %v", dupResp)
	}

	// 3. Cancela a reserva distribuída a partir da Empresa A
	cancelReq := protocol.CancelReservationRequest{
		ReservationID: res.ID,
	}

	cancelResp, err := paxClient.Send(protocol.ActionCancelReservation, cancelReq)
	if err != nil {
		t.Fatalf("falha ao enviar cancel_reservation: %v", err)
	}
	if cancelResp.Success != "success" {
		t.Fatalf("cancelamento falhou: %s", cancelResp.Message)
	}

	// Aguarda processamento assíncrono do release HTTP remoto
	time.Sleep(200 * time.Millisecond)

	// 4. Valida devolução dos assentos em ambas as empresas
	rACancel, _ := cluster.Nodes["empresa-a"].Server.Repository().GetRideByID(rideA.ID)
	rBCancel, _ := cluster.Nodes["empresa-b"].Server.Repository().GetRideByID(rideB.ID)

	if rACancel.Segments[0].AvailableSeats != 2 {
		t.Fatalf("esperava 2 assentos restaurados na empresa-a, obteve %d", rACancel.Segments[0].AvailableSeats)
	}
	if rBCancel.Segments[0].AvailableSeats != 2 {
		t.Fatalf("esperava 2 assentos restaurados na empresa-b, obteve %d", rBCancel.Segments[0].AvailableSeats)
	}
}
