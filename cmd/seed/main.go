package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/client"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
)

const (
	seedPassword = "12345678Abc#"
	dateLayout   = "02/01/2006"
)

type seedUser struct {
	name  string
	email string
	role  models.UserRole
}

func main() {
	address := flag.String("addr", "localhost:8080", "endereço do servidor TCP")
	dateText := flag.String("date", time.Now().AddDate(0, 0, 1).Format(dateLayout), "data das caronas, no formato dd/mm/aaaa")
	flag.Parse()

	date, err := time.ParseInLocation(dateLayout, *dateText, time.Local)
	if err != nil {
		log.Fatalf("Data inválida: %v", err)
	}

	drivers := []seedUser{
		{name: "Ana Motorista", email: "ana@gmail.com", role: models.RoleDriver},
		{name: "Bruno Motorista", email: "bruno@gmail.com", role: models.RoleDriver},
	}
	passengers := []seedUser{
		{name: "Alice Passageira", email: "alice@gmail.com", role: models.RolePassenger},
		{name: "Beto Passageiro", email: "beto@gmail.com", role: models.RolePassenger},
	}

	for _, passenger := range passengers {
		if err := registerUser(*address, passenger); err != nil {
			log.Fatal(err)
		}
	}
	if err := seedDriver(*address, drivers[0], protocol.CreateVehicleRequest{
		Plate: "ANA-2026", Model: "Hatch", Color: "Azul", SeatCapacity: 4,
	}, anaRide(date)); err != nil {
		log.Fatal(err)
	}
	if err := seedDriver(*address, drivers[1], protocol.CreateVehicleRequest{
		Plate: "BRU-2026", Model: "Sedan", Color: "Prata", SeatCapacity: 4,
	}, brunoRide(date)); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Dados de teste disponíveis para %s.\n", date.Format(dateLayout))
	fmt.Println("Motoristas: ana@gmail.com e bruno@gmail.com")
	fmt.Println("Passageiros: alice@gmail.com e beto@gmail.com")
	fmt.Printf("Senha de todas as contas: %s\n", seedPassword)
	fmt.Println("Demonstração de conexão: Alagoinhas -> Feira de Santana com Ana; Feira de Santana -> Lauro de Freitas com Bruno.")
}

func registerUser(address string, user seedUser) error {
	tcpClient, err := client.Dial(address)
	if err != nil {
		return fmt.Errorf("conectar para cadastrar %s: %w", user.email, err)
	}
	defer tcpClient.Close()

	response, err := send(tcpClient, protocol.ActionRegisterUser, protocol.CreateUserRequest{
		Name: user.name, Email: user.email, Password: seedPassword, Role: user.role,
	})
	if err != nil {
		return fmt.Errorf("cadastrar %s: %w", user.email, err)
	}
	if response.Success == "success" {
		fmt.Printf("Conta criada: %s\n", user.email)
		return nil
	}

	// O comando pode ser executado mais de uma vez. Se a conta já existir,
	// ela é mantida e as etapas seguintes usam o login normalmente.
	fmt.Printf("Conta mantida: %s (%s)\n", user.email, response.Message)
	return nil
}

func seedDriver(address string, driver seedUser, vehicle protocol.CreateVehicleRequest, ride protocol.CreateRideRequest) error {
	if err := registerUser(address, driver); err != nil {
		return err
	}

	tcpClient, err := client.Dial(address)
	if err != nil {
		return fmt.Errorf("conectar para preparar motorista %s: %w", driver.email, err)
	}
	defer tcpClient.Close()

	if response, err := send(tcpClient, protocol.ActionLogin, protocol.LoginRequest{Email: driver.email, Password: seedPassword}); err != nil {
		return fmt.Errorf("login de %s: %w", driver.email, err)
	} else if response.Success != "success" {
		return fmt.Errorf("login de %s recusado: %s", driver.email, response.Message)
	}

	if err := ensureVehicle(tcpClient, vehicle); err != nil {
		return fmt.Errorf("veículo de %s: %w", driver.email, err)
	}
	if err := ensureRide(tcpClient, ride); err != nil {
		return fmt.Errorf("carona de %s: %w", driver.email, err)
	}

	return nil
}

func ensureVehicle(tcpClient *client.TCPClient, vehicle protocol.CreateVehicleRequest) error {
	response, err := send(tcpClient, protocol.ActionGetMyVehicle, nil)
	if err != nil {
		return err
	}
	if response.Success == "success" {
		fmt.Println("Veículo de teste já existe.")
		return nil
	}

	response, err = send(tcpClient, protocol.ActionRegisterVehicle, vehicle)
	if err != nil {
		return err
	}
	if response.Success != "success" {
		return errors.New(response.Message)
	}
	fmt.Println("Veículo de teste criado.")
	return nil
}

func ensureRide(tcpClient *client.TCPClient, ride protocol.CreateRideRequest) error {
	response, err := send(tcpClient, protocol.ActionListMyRides, nil)
	if err != nil {
		return err
	}
	if response.Success != "success" {
		return errors.New(response.Message)
	}

	var rides []models.Ride
	if err := json.Unmarshal(response.Payload, &rides); err != nil {
		return fmt.Errorf("interpretar caronas existentes: %w", err)
	}
	for _, existingRide := range rides {
		if sameSeedRide(existingRide, ride) {
			fmt.Println("Carona de teste mantida: a mesma rota já está publicada.")
			return nil
		}
	}

	response, err = send(tcpClient, protocol.ActionCreateRide, ride)
	if err != nil {
		return err
	}
	if response.Success != "success" {
		return errors.New(response.Message)
	}
	fmt.Println("Carona de teste criada.")
	return nil
}

func sameSeedRide(existing models.Ride, expected protocol.CreateRideRequest) bool {
	if !existing.DepartureAt.Equal(expected.DepartureAt) || len(existing.Segments) != len(expected.Segments) {
		return false
	}

	for index, expectedStage := range expected.Segments {
		existingStage := existing.Segments[index]
		if existingStage.Origin != expectedStage.Origin ||
			existingStage.Destination != expectedStage.Destination ||
			!existingStage.DepartureAt.Equal(expectedStage.DepartureAt) ||
			!existingStage.ArrivalAt.Equal(expectedStage.ArrivalAt) ||
			existingStage.PriceCents != expectedStage.PriceCents {
			return false
		}
	}

	return true
}

func send(tcpClient *client.TCPClient, action protocol.Action, body any) (protocol.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return protocol.Response{}, err
	}
	return tcpClient.Send(protocol.Request{Action: action, Payload: payload})
}

func anaRide(date time.Time) protocol.CreateRideRequest {
	departure := time.Date(date.Year(), date.Month(), date.Day(), 7, 0, 0, 0, time.Local)
	arrivalFeira := departure.Add(60 * time.Minute)
	departureFeira := arrivalFeira.Add(15 * time.Minute)
	arrivalVitoria := departureFeira.Add(4 * time.Hour)
	return protocol.CreateRideRequest{
		DepartureAt: departure,
		Segments: []protocol.CreateStageRequest{
			{Origin: enum.Alagoinhas, Destination: enum.FeiraDeSantana, DepartureAt: departure, ArrivalAt: arrivalFeira, PriceCents: 2000, AvailableSeats: 3},
			{Origin: enum.FeiraDeSantana, Destination: enum.VitoriaDaConquista, DepartureAt: departureFeira, ArrivalAt: arrivalVitoria, PriceCents: 6500, AvailableSeats: 3},
		},
	}
}

func brunoRide(date time.Time) protocol.CreateRideRequest {
	departure := time.Date(date.Year(), date.Month(), date.Day(), 6, 0, 0, 0, time.Local)
	arrivalFeira := departure.Add(2*time.Hour + 30*time.Minute)
	departureFeira := arrivalFeira.Add(15 * time.Minute)
	arrivalLauro := departureFeira.Add(90 * time.Minute)
	return protocol.CreateRideRequest{
		DepartureAt: departure,
		Segments: []protocol.CreateStageRequest{
			{Origin: enum.Jequie, Destination: enum.FeiraDeSantana, DepartureAt: departure, ArrivalAt: arrivalFeira, PriceCents: 4500, AvailableSeats: 3},
			{Origin: enum.FeiraDeSantana, Destination: enum.LauroDeFreitas, DepartureAt: departureFeira, ArrivalAt: arrivalLauro, PriceCents: 3000, AvailableSeats: 3},
		},
	}
}
