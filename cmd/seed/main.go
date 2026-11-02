package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"
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

type serverTarget struct {
	companyID string
	address   string
}

func main() {
	address := flag.String("addr", "", "endereço do servidor TCP único (ex: localhost:8080)")
	serversFlag := flag.String("servers", "", "lista de servidores das empresas (ex: empresa-a=localhost:8080,empresa-b=localhost:8082,empresa-c=localhost:8084)")
	dateText := flag.String("date", "", "data das caronas (dd/mm/aaaa). Se vazio, popula automaticamente para HOJE e AMANHÃ")
	flag.Parse()

	var dates []time.Time
	if *dateText != "" {
		d, err := time.ParseInLocation(dateLayout, *dateText, time.Local)
		if err != nil {
			log.Fatalf("Data inválida: %v", err)
		}
		dates = append(dates, d)
	} else {
		// Por padrão, popula para HOJE e para AMANHÃ para que os testes funcionem sem depender do dia/hora!
		today := time.Now()
		tomorrow := today.AddDate(0, 0, 1)
		dates = append(dates, today, tomorrow)
	}

	targets := parseTargets(*address, *serversFlag)
	if len(targets) == 0 {
		targets = []serverTarget{{companyID: "empresa-a", address: "localhost:8080"}}
	}

	fmt.Printf("====================================================================\n")
	fmt.Printf(" INICIANDO POVOAMENTO DE DADOS DE TESTE (CONSORCIO VAIJUNTO)\n")
	fmt.Printf(" Senha padrao de todas as contas: %s\n", seedPassword)
	for _, d := range dates {
		fmt.Printf("   -> Caronas sendo criadas para a data: %s\n", d.Format(dateLayout))
	}
	fmt.Printf("====================================================================\n\n")

	for _, d := range dates {
		if len(targets) > 1 {
			seedConsortium(targets, d)
		} else {
			seedSingleServer(targets[0].address, d)
		}
	}

	fmt.Println("\n====================================================================")
	fmt.Println(" ROTEIROS DE DEMONSTRACAO PREPARADOS:")
	fmt.Println(" 1. CARONAS CONCORRENTES (Mesmo trecho em empresas diferentes):")
	fmt.Println("    - Feira de Santana -> Salvador:")
	fmt.Println("      * Empresa A (Ana): R$ 35,00 (08:00)")
	fmt.Println("      * Empresa B (Bruno): R$ 30,00 (08:15)")
	fmt.Println("      * Empresa C (Carlos): R$ 25,00 (08:30)")
	fmt.Println("    -> Na busca, o passageiro verá as 3 empresas lado a lado!")
	fmt.Println()
	fmt.Println(" 2. TRECHOS COMPLEMENTARES (Itinerario 2PC entre 2 empresas):")
	fmt.Println("    - Salvador -> Lencois:")
	fmt.Println("      * Trecho 1: Salvador -> Feira de Santana (Empresa A)")
	fmt.Println("      * Trecho 2: Feira de Santana -> Lencois (Empresa B)")
	fmt.Println("    -> Dispara o 2PC entre Empresa A e Empresa B!")
	fmt.Println()
	fmt.Println(" 3. VIAGEM INTEGRADA TOTAL (Itinerario 2PC entre as 3 empresas):")
	fmt.Println("    - Salvador -> Vitoria da Conquista (via Lencois):")
	fmt.Println("      * Trecho 1: Salvador -> Feira de Santana (Empresa A)")
	fmt.Println("      * Trecho 2: Feira de Santana -> Lencois (Empresa B)")
	fmt.Println("      * Trecho 3: Lencois -> Vitoria da Conquista (Empresa C)")
	fmt.Println("    -> Dispara o 2PC triangular entre Empresa A, B e C!")
	fmt.Println("====================================================================")
}

func parseTargets(singleAddr, serversRaw string) []serverTarget {
	var targets []serverTarget

	if serversRaw != "" {
		parts := strings.Split(serversRaw, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			sub := strings.SplitN(part, "=", 2)
			if len(sub) == 2 {
				targets = append(targets, serverTarget{
					companyID: strings.TrimSpace(sub[0]),
					address:   strings.TrimSpace(sub[1]),
				})
			} else {
				targets = append(targets, serverTarget{
					companyID: fmt.Sprintf("servidor-%d", len(targets)+1),
					address:   sub[0],
				})
			}
		}
		return targets
	}

	if singleAddr != "" {
		parts := strings.Split(singleAddr, ",")
		for idx, addr := range parts {
			addr = strings.TrimSpace(addr)
			if addr != "" {
				compID := "empresa-a"
				if idx == 1 {
					compID = "empresa-b"
				} else if idx == 2 {
					compID = "empresa-c"
				}
				targets = append(targets, serverTarget{
					companyID: compID,
					address:   addr,
				})
			}
		}
	}

	return targets
}

func seedConsortium(targets []serverTarget, date time.Time) {
	for _, target := range targets {
		fmt.Printf("\n--- Populando %s em %s ---\n", target.companyID, target.address)

		switch target.companyID {
		case "empresa-a":
			seedEmpresaA(target.address, date)
		case "empresa-b":
			seedEmpresaB(target.address, date)
		case "empresa-c":
			seedEmpresaC(target.address, date)
		default:
			// Servidor genérico
			seedGeneric(target.address, target.companyID, date)
		}
	}
}

func seedEmpresaA(address string, date time.Time) {
	_ = registerUser(address, seedUser{name: "Alice Passageira", email: "alice@gmail.com", role: models.RolePassenger})

	ana := seedUser{name: "Ana Motorista", email: "ana@empresa-a.com", role: models.RoleDriver}
	anaVeh := protocol.CreateVehicleRequest{Plate: "ANA-1A01", Model: "Corolla", Color: "Branco", SeatCapacity: 4}

	// Carona 1: Salvador -> Feira de Santana -> Alagoinhas (alimenta conexões para B e C)
	dep1 := time.Date(date.Year(), date.Month(), date.Day(), 7, 0, 0, 0, time.Local)
	arrFeira1 := dep1.Add(90 * time.Minute)
	depFeira1 := arrFeira1.Add(15 * time.Minute)
	arrAla1 := depFeira1.Add(60 * time.Minute)
	ride1 := protocol.CreateRideRequest{
		DepartureAt: dep1,
		Segments: []protocol.CreateStageRequest{
			{Origin: enum.Salvador, Destination: enum.FeiraDeSantana, DepartureAt: dep1, ArrivalAt: arrFeira1, PriceCents: 3500, AvailableSeats: 4},
			{Origin: enum.FeiraDeSantana, Destination: enum.Alagoinhas, DepartureAt: depFeira1, ArrivalAt: arrAla1, PriceCents: 2000, AvailableSeats: 4},
		},
	}
	_ = seedDriver(address, ana, anaVeh, ride1)

	// Carona 2 (Concorrente direta Feira -> Salvador): R$ 35,00
	dep2 := time.Date(date.Year(), date.Month(), date.Day(), 8, 0, 0, 0, time.Local)
	arr2 := dep2.Add(90 * time.Minute)
	ride2 := protocol.CreateRideRequest{
		DepartureAt: dep2,
		Segments: []protocol.CreateStageRequest{
			{Origin: enum.FeiraDeSantana, Destination: enum.Salvador, DepartureAt: dep2, ArrivalAt: arr2, PriceCents: 3500, AvailableSeats: 4},
		},
	}
	_ = ensureRideOnAddress(address, ana, ride2)
}

func seedEmpresaB(address string, date time.Time) {
	_ = registerUser(address, seedUser{name: "Beto Passageiro", email: "beto@gmail.com", role: models.RolePassenger})

	bruno := seedUser{name: "Bruno Motorista", email: "bruno@empresa-b.com", role: models.RoleDriver}
	brunoVeh := protocol.CreateVehicleRequest{Plate: "BRU-2B02", Model: "Civic", Color: "Prata", SeatCapacity: 4}

	// Carona 1: Feira de Santana -> Lençóis (conecta com a Carona 1 da Ana que chega às 08:30 em Feira!)
	dep1 := time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, time.Local)
	arrLencois1 := dep1.Add(4 * time.Hour)
	ride1 := protocol.CreateRideRequest{
		DepartureAt: dep1,
		Segments: []protocol.CreateStageRequest{
			{Origin: enum.FeiraDeSantana, Destination: enum.Lencois, DepartureAt: dep1, ArrivalAt: arrLencois1, PriceCents: 6500, AvailableSeats: 4},
		},
	}
	_ = seedDriver(address, bruno, brunoVeh, ride1)

	// Carona 2 (Concorrente direta Feira -> Salvador): R$ 30,00 (mais barata que a Empresa A!)
	dep2 := time.Date(date.Year(), date.Month(), date.Day(), 8, 15, 0, 0, time.Local)
	arr2 := dep2.Add(90 * time.Minute)
	ride2 := protocol.CreateRideRequest{
		DepartureAt: dep2,
		Segments: []protocol.CreateStageRequest{
			{Origin: enum.FeiraDeSantana, Destination: enum.Salvador, DepartureAt: dep2, ArrivalAt: arr2, PriceCents: 3000, AvailableSeats: 4},
		},
	}
	_ = ensureRideOnAddress(address, bruno, ride2)
}

func seedEmpresaC(address string, date time.Time) {
	_ = registerUser(address, seedUser{name: "Carla Passageira", email: "carla@gmail.com", role: models.RolePassenger})

	carlos := seedUser{name: "Carlos Motorista", email: "carlos@empresa-c.com", role: models.RoleDriver}
	carlosVeh := protocol.CreateVehicleRequest{Plate: "CAR-3C03", Model: "Renegade", Color: "Preto", SeatCapacity: 4}

	// Carona 1: Lençóis -> Vitória da Conquista (conecta com a Carona do Bruno que chega às 13:00 em Lençóis!)
	dep1 := time.Date(date.Year(), date.Month(), date.Day(), 14, 0, 0, 0, time.Local)
	arrVitoria1 := dep1.Add(4 * time.Hour)
	ride1 := protocol.CreateRideRequest{
		DepartureAt: dep1,
		Segments: []protocol.CreateStageRequest{
			{Origin: enum.Lencois, Destination: enum.VitoriaDaConquista, DepartureAt: dep1, ArrivalAt: arrVitoria1, PriceCents: 5500, AvailableSeats: 4},
		},
	}
	_ = seedDriver(address, carlos, carlosVeh, ride1)

	// Carona 2 (Concorrente direta Feira -> Salvador): R$ 25,00 (a mais barata do consórcio!)
	dep2 := time.Date(date.Year(), date.Month(), date.Day(), 8, 30, 0, 0, time.Local)
	arr2 := dep2.Add(90 * time.Minute)
	ride2 := protocol.CreateRideRequest{
		DepartureAt: dep2,
		Segments: []protocol.CreateStageRequest{
			{Origin: enum.FeiraDeSantana, Destination: enum.Salvador, DepartureAt: dep2, ArrivalAt: arr2, PriceCents: 2500, AvailableSeats: 4},
		},
	}
	_ = ensureRideOnAddress(address, carlos, ride2)
}

func seedGeneric(address, compID string, date time.Time) {
	drv := seedUser{name: fmt.Sprintf("Motorista %s", compID), email: fmt.Sprintf("driver@%s.test", compID), role: models.RoleDriver}
	veh := protocol.CreateVehicleRequest{Plate: "GEN-9999", Model: "Sedan", Color: "Cinza", SeatCapacity: 4}
	dep := time.Date(date.Year(), date.Month(), date.Day(), 8, 0, 0, 0, time.Local)
	arr := dep.Add(2 * time.Hour)
	ride := protocol.CreateRideRequest{
		DepartureAt: dep,
		Segments: []protocol.CreateStageRequest{
			{Origin: enum.FeiraDeSantana, Destination: enum.Salvador, DepartureAt: dep, ArrivalAt: arr, PriceCents: 3000, AvailableSeats: 4},
		},
	}
	_ = seedDriver(address, drv, veh, ride)
}

func seedSingleServer(address string, date time.Time) {
	fmt.Printf("Populando servidor unico em %s...\n", address)
	drivers := []seedUser{
		{name: "Ana Motorista", email: "ana@gmail.com", role: models.RoleDriver},
		{name: "Bruno Motorista", email: "bruno@gmail.com", role: models.RoleDriver},
	}
	passengers := []seedUser{
		{name: "Alice Passageira", email: "alice@gmail.com", role: models.RolePassenger},
		{name: "Beto Passageiro", email: "beto@gmail.com", role: models.RolePassenger},
	}

	for _, p := range passengers {
		_ = registerUser(address, p)
	}

	_ = seedDriver(address, drivers[0], protocol.CreateVehicleRequest{
		Plate: "ANA-2026", Model: "Hatch", Color: "Azul", SeatCapacity: 4,
	}, anaRide(date))

	_ = seedDriver(address, drivers[1], protocol.CreateVehicleRequest{
		Plate: "BRU-2026", Model: "Sedan", Color: "Prata", SeatCapacity: 4,
	}, brunoRide(date))
}

func registerUser(address string, user seedUser) error {
	tcpClient, err := client.Dial(address)
	if err != nil {
		return fmt.Errorf("conectar para cadastrar %s em %s: %w", user.email, address, err)
	}
	defer tcpClient.Close()

	response, err := send(tcpClient, protocol.ActionRegisterUser, protocol.CreateUserRequest{
		Name: user.name, Email: user.email, Password: seedPassword, Role: user.role,
	})
	if err != nil {
		return fmt.Errorf("cadastrar %s: %w", user.email, err)
	}
	if response.Success == "success" {
		fmt.Printf("  [+] Conta criada: %s\n", user.email)
		return nil
	}

	fmt.Printf("  [~] Conta mantida: %s (%s)\n", user.email, response.Message)
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
		return fmt.Errorf("veiculo de %s: %w", driver.email, err)
	}
	if err := ensureRide(tcpClient, ride); err != nil {
		return fmt.Errorf("carona de %s: %w", driver.email, err)
	}

	return nil
}

func ensureRideOnAddress(address string, driver seedUser, ride protocol.CreateRideRequest) error {
	tcpClient, err := client.Dial(address)
	if err != nil {
		return err
	}
	defer tcpClient.Close()

	if response, err := send(tcpClient, protocol.ActionLogin, protocol.LoginRequest{Email: driver.email, Password: seedPassword}); err != nil {
		return err
	} else if response.Success != "success" {
		return errors.New(response.Message)
	}

	return ensureRide(tcpClient, ride)
}

func ensureVehicle(tcpClient *client.TCPClient, vehicle protocol.CreateVehicleRequest) error {
	response, err := send(tcpClient, protocol.ActionGetMyVehicle, nil)
	if err != nil {
		return err
	}
	if response.Success == "success" {
		fmt.Printf("  [~] Veiculo ja existente: %s\n", vehicle.Plate)
		return nil
	}

	response, err = send(tcpClient, protocol.ActionRegisterVehicle, vehicle)
	if err != nil {
		return err
	}
	if response.Success != "success" {
		return errors.New(response.Message)
	}
	fmt.Printf("  [+] Veiculo cadastrado: %s (%s)\n", vehicle.Plate, vehicle.Model)
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
			fmt.Printf("  [~] Carona ja publicada (%s -> %s)\n", ride.Segments[0].Origin, ride.Segments[len(ride.Segments)-1].Destination)
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
	fmt.Printf("  [+] Carona publicada: %s -> %s (partida: %s)\n",
		ride.Segments[0].Origin,
		ride.Segments[len(ride.Segments)-1].Destination,
		ride.DepartureAt.Format("15:04"),
	)
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
