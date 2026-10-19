package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/consortium"
	"github.com/JoaoAnunciacaoDev/PBL---VaiJunto-Concorrencia-E-Conectividade/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RepositoryReader define a interface de acesso e modificação do repositório
// necessária para os endpoints HTTP inter-servidor.
type RepositoryReader interface {
	GetActiveRidesByDate(date time.Time) ([]*models.Ride, error)
	GetRideByID(id uuid.UUID) (*models.Ride, error)
	DecrementSeat(rideID, segmentID uuid.UUID) error
	IncrementSeat(rideID, segmentID uuid.UUID) error
}

// HTTPServer encapsula o servidor REST em Gin para comunicação inter-servidores.
type HTTPServer struct {
	companyID  string
	addr       string
	repo       RepositoryReader
	lockStore  *consortium.LockStore
	wal        *consortium.WAL
	clock      *consortium.LamportClock
	engine     *gin.Engine
	httpServer *http.Server
	stopChan   chan struct{}
}

// NewHTTPServer cria e configura a instância do servidor HTTP com suas rotas internas.
func NewHTTPServer(
	companyID, addr string,
	repo RepositoryReader,
	lockStore *consortium.LockStore,
	wal *consortium.WAL,
	clock *consortium.LamportClock,
) *HTTPServer {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	s := &HTTPServer{
		companyID: companyID,
		addr:      addr,
		repo:      repo,
		lockStore: lockStore,
		wal:       wal,
		clock:     clock,
		engine:    engine,
		stopChan:  make(chan struct{}),
	}

	s.setupRoutes()
	return s
}

func (s *HTTPServer) setupRoutes() {
	v1 := s.engine.Group("/internal/v1")
	{
		v1.GET("/health", s.handleHealth)
		v1.GET("/rides", s.handleGetRides)

		// Endpoints do protocolo 2PC (Two-Phase Commit)
		v1.POST("/segments/:segmentID/prepare", s.handlePrepare)
		v1.POST("/segments/:segmentID/commit", s.handleCommit)
		v1.POST("/segments/:segmentID/abort", s.handleAbort)
		v1.GET("/segments/:segmentID/status/:txID", s.handleStatus)
	}
}

// Start inicia o servidor HTTP e a goroutine de expiração de locks por TTL.
func (s *HTTPServer) Start() error {
	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: s.engine,
	}

	go s.startLockReaper()

	log.Printf("[httpapi] servidor REST inter-empresa iniciado empresa=%s addr=%s", s.companyID, s.addr)
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Stop encerra o servidor HTTP graciosamente.
func (s *HTTPServer) Stop(ctx context.Context) error {
	close(s.stopChan)
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// Engine expõe o engine do Gin para testes automatizados.
func (s *HTTPServer) Engine() *gin.Engine {
	return s.engine
}

// startLockReaper varre periodicamente locks expirados e os registra no WAL.
func (s *HTTPServer) startLockReaper() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			expired := s.lockStore.ExpireAll()
			for _, lock := range expired {
				log.Printf("[httpapi] lock expirado por TTL tx_id=%s segment_id=%s", lock.TxID, lock.SegmentID)
				_ = s.wal.Append(consortium.WALEntry{
					TxID:          lock.TxID,
					Role:          "participant",
					Status:        consortium.TxExpired,
					CoordinatorID: s.companyID,
					SegmentIDs:    []string{lock.SegmentID},
					UpdatedAt:     time.Now(),
				})
			}
		}
	}
}
