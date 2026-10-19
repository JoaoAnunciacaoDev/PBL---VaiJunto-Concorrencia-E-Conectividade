package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/client"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/enum"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/protocol"
	"github.com/google/uuid"
)

type companyTarget struct {
	name string
	addr string
}

type metrics struct {
	totalAttempts    atomic.Int64
	confirmed        atomic.Int64
	conflicts        atomic.Int64
	noItineraries    atomic.Int64
	networkErrors    atomic.Int64
	authErrors       atomic.Int64

	mu        sync.Mutex
	latencies []time.Duration
}

func (m *metrics) recordLatency(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latencies = append(m.latencies, d)
}

func main() {
	companiesFlag := flag.String("companies", "empresa-a=127.0.0.1:8080,empresa-b=127.0.0.1:8082,empresa-c=127.0.0.1:8084", "empresas e endereços TCP (nome=host:porta)")
	passengersFlag := flag.Int("passengers", 30, "número de passageiros virtuais concorrentes")
	durationFlag := flag.Duration("duration", 15*time.Second, "duração total da simulação")
	dateFlag := flag.String("date", time.Now().Format("2006-01-02"), "data para busca de caronas (YYYY-MM-DD)")
	originFlag := flag.Int("origin", 0, "ID da cidade de origem (0 para aleatório)")
	destFlag := flag.Int("dest", 0, "ID da cidade de destino (0 para aleatório)")
	flag.Parse()

	companies := parseCompanies(*companiesFlag)
	if len(companies) == 0 {
		fmt.Fprintf(os.Stderr, "Nenhuma empresa configurada.\n")
		os.Exit(1)
	}

	searchDate, err := time.Parse("2006-01-02", *dateFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Data inválida %q: %v\n", *dateFlag, err)
		os.Exit(1)
	}

	fmt.Println("======================================================================")
	fmt.Println("    SIMULADOR AUTÔNOMO DE CARGAS DISTRIBUÍDAS - VAIJUNTO (PBL 2)     ")
	fmt.Println("======================================================================")
	fmt.Printf("Empresas Alvo: %d\n", len(companies))
	for _, c := range companies {
		fmt.Printf("  • %-12s -> %s\n", c.name, c.addr)
	}
	fmt.Printf("Passageiros Concorrentes: %d\n", *passengersFlag)
	fmt.Printf("Duração:                  %v\n", *durationFlag)
	fmt.Printf("Data de Busca:            %s\n", searchDate.Format("2006-01-02"))
	fmt.Println("----------------------------------------------------------------------")
	fmt.Println("Iniciando geração de carga... Pressione Ctrl+C para encerrar antecipadamente.")

	ctx, cancel := context.WithTimeout(context.Background(), *durationFlag)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nInterrupção recebida, finalizando simulação...")
		cancel()
	}()

	var met metrics
	startTime := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < *passengersFlag; i++ {
		wg.Add(1)
		target := companies[i%len(companies)]
		go func(workerID int, target companyTarget) {
			defer wg.Done()
			runPassengerWorker(ctx, workerID, target, searchDate, enum.City(*originFlag), enum.City(*destFlag), &met)
		}(i+1, target)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	printReport(&met, totalDuration)
}

func runPassengerWorker(
	ctx context.Context,
	workerID int,
	target companyTarget,
	date time.Time,
	fixedOrigin, fixedDest enum.City,
	m *metrics,
) {
	tcpClient, err := client.Dial(target.addr)
	if err != nil {
		m.networkErrors.Add(1)
		return
	}
	defer tcpClient.Close()

	// 1. Cadastra passageiro exclusivo do worker
	email := fmt.Sprintf("sim-w%d-%s@vaijunto.test", workerID, uuid.New().String()[:8])
	password := "Senha@123"

	regPayload, _ := json.Marshal(protocol.CreateUserRequest{
		Name:     fmt.Sprintf("Simulador Worker %d", workerID),
		Email:    email,
		Password: password,
		Role:     models.RolePassenger,
	})
	regResp, err := tcpClient.Send(protocol.Request{
		Action:  protocol.ActionRegisterUser,
		Payload: regPayload,
	})
	if err != nil || regResp.Success != "success" {
		m.authErrors.Add(1)
		return
	}

	// 2. Login
	loginPayload, _ := json.Marshal(protocol.LoginRequest{
		Email:    email,
		Password: password,
	})
	loginResp, err := tcpClient.Send(protocol.Request{
		Action:  protocol.ActionLogin,
		Payload: loginPayload,
	})
	if err != nil || loginResp.Success != "success" {
		m.authErrors.Add(1)
		return
	}

	cities := enum.Cities()
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)*1000))

	// 3. Loop contínuo de busca e confirmação sob concorrência
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		origin := fixedOrigin
		dest := fixedDest
		if !origin.IsValid() || !dest.IsValid() || origin == dest {
			origin = cities[rng.Intn(len(cities))]
			dest = cities[rng.Intn(len(cities))]
			for dest == origin {
				dest = cities[rng.Intn(len(cities))]
			}
		}

		// Busca itinerários no consórcio
		searchPayload, _ := json.Marshal(protocol.SearchItinerariesRequest{
			Origin:      origin,
			Destination: dest,
			Date:        date,
		})
		searchResp, err := tcpClient.Send(protocol.Request{
			Action:  protocol.ActionSearchItineraries,
			Payload: searchPayload,
		})
		if err != nil {
			m.networkErrors.Add(1)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if searchResp.Success != "success" {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		var itineraries []protocol.ItineraryResponse
		if err := json.Unmarshal(searchResp.Payload, &itineraries); err != nil || len(itineraries) == 0 {
			m.noItineraries.Add(1)
			time.Sleep(50 * time.Millisecond)
			continue
		}

		// Escolhe um itinerário aleatório e disputa a reserva
		chosen := itineraries[rng.Intn(len(itineraries))]
		segments := make([]models.ReservedSegment, 0, len(chosen.Segments))
		for _, seg := range chosen.Segments {
			segments = append(segments, models.ReservedSegment{
				RideID:    seg.RideID,
				SegmentID: seg.SegmentID,
				CompanyID: seg.CompanyID,
			})
		}

		confirmPayload, _ := json.Marshal(protocol.ConfirmReservationRequest{
			Segments: segments,
		})

		m.totalAttempts.Add(1)
		reqStart := time.Now()

		confirmResp, err := tcpClient.Send(protocol.Request{
			Action:  protocol.ActionConfirmReservation,
			Payload: confirmPayload,
		})
		reqDuration := time.Since(reqStart)

		if err != nil {
			m.networkErrors.Add(1)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		m.recordLatency(reqDuration)

		if confirmResp.Success == "success" {
			m.confirmed.Add(1)
		} else {
			// Erro de assento esgotado, disputa simultânea (conflito 409) ou servidor parceiro
			m.conflicts.Add(1)
		}

		// Breve intervalo para alternância de escalonamento das goroutines
		time.Sleep(10 * time.Millisecond)
	}
}

func parseCompanies(raw string) []companyTarget {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	var list []companyTarget
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 {
			list = append(list, companyTarget{
				name: strings.TrimSpace(kv[0]),
				addr: strings.TrimSpace(kv[1]),
			})
		}
	}
	return list
}

func printReport(m *metrics, duration time.Duration) {
	attempts := m.totalAttempts.Load()
	confirmed := m.confirmed.Load()
	conflicts := m.conflicts.Load()
	noItin := m.noItineraries.Load()
	netErr := m.networkErrors.Load()
	authErr := m.authErrors.Load()

	fmt.Println("\n======================================================================")
	fmt.Println("                   RELATÓRIO FINAL DA SIMULAÇÃO                       ")
	fmt.Println("======================================================================")
	fmt.Printf("Tempo de Execução:             %.2fs\n", duration.Seconds())
	fmt.Printf("Tentativas de Reserva (2PC):   %d\n", attempts)
	fmt.Printf("  • Reservas Confirmadas:      %d\n", confirmed)
	fmt.Printf("  • Conflitos / Vagas Esgotadas: %d\n", conflicts)
	fmt.Printf("Buscas Sem Itinerário:         %d\n", noItin)
	fmt.Printf("Erros de Conexão/Rede:         %d\n", netErr)
	fmt.Printf("Erros de Autenticação:         %d\n", authErr)

	if duration.Seconds() > 0 {
		throughput := float64(attempts) / duration.Seconds()
		fmt.Printf("Throughput Médio:              %.2f reservas/s\n", throughput)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.latencies) > 0 {
		sort.Slice(m.latencies, func(i, j int) bool {
			return m.latencies[i] < m.latencies[j]
		})

		var sum time.Duration
		for _, l := range m.latencies {
			sum += l
		}
		avg := sum / time.Duration(len(m.latencies))
		p50 := m.latencies[len(m.latencies)*50/100]
		p95 := m.latencies[len(m.latencies)*95/100]
		p99 := m.latencies[len(m.latencies)*99/100]
		maxLat := m.latencies[len(m.latencies)-1]

		fmt.Println("----------------------------------------------------------------------")
		fmt.Println("Métricas de Latência (Confirmação 2PC):")
		fmt.Printf("  • Média:                     %v\n", avg)
		fmt.Printf("  • P50:                       %v\n", p50)
		fmt.Printf("  • P95:                       %v\n", p95)
		fmt.Printf("  • P99:                       %v\n", p99)
		fmt.Printf("  • Máxima:                    %v\n", maxLat)
	}
	fmt.Println("======================================================================")
}
