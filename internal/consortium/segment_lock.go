package consortium

import (
	"errors"
	"sync"
	"time"
)

const (
	// DefaultLockTTL é o tempo padrão de tolerância (10s).
	DefaultLockTTL = 10 * time.Second
)

// SegmentLock representa a pré-reserva de um assento em um trecho específico.
type SegmentLock struct {
	TxID          string    `json:"tx_id"`
	CoordinatorID string    `json:"coordinator_id"`
	LamportClock  uint64    `json:"lamport_clock"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// IsExpired verifica se o lock ultrapassou sua data limite.
func (l *SegmentLock) IsExpired(now time.Time) bool {
	return !l.ExpiresAt.After(now)
}

// ExpiredLock descreve um lock que acabou de expirar no store.
type ExpiredLock struct {
	SegmentID string
	TxID      string
}

// LockStore gerencia os locks de pré-reserva (PREPARED) por segmento.
type LockStore struct {
	mu         sync.Mutex
	locks      map[string]SegmentLock // chave: segmentID
	defaultTTL time.Duration
}

// NewLockStore cria um repositório de locks em memória com TTL configurável.
func NewLockStore(defaultTTL time.Duration) *LockStore {
	if defaultTTL <= 0 {
		defaultTTL = DefaultLockTTL
	}
	return &LockStore{
		locks:      make(map[string]SegmentLock),
		defaultTTL: defaultTTL,
	}
}

// isBetterPriority define a ordem total de precedência de Lamport com desempate
// determinístico por CompanyID: menor (Lamport, CompanyID) vence.
func isBetterPriority(newClock uint64, newCoord string, curClock uint64, curCoord string) bool {
	if newClock != curClock {
		return newClock < curClock
	}
	return newCoord < curCoord
}

// TryAcquire tenta obter o lock para o trecho.
// Retorna:
// - acquired: se a transação obteve o lock
// - evictedTxID: id da transação que foi desbancada por ordem de precedência, se houver
// - err: erro de validação
func (s *LockStore) TryAcquire(segmentID string, lock SegmentLock) (bool, string, error) {
	if segmentID == "" || lock.TxID == "" || lock.CoordinatorID == "" {
		return false, "", errors.New("parâmetros de lock inválidos")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if lock.ExpiresAt.IsZero() {
		lock.ExpiresAt = now.Add(s.defaultTTL)
	}

	current, exists := s.locks[segmentID]
	if !exists || current.IsExpired(now) {
		s.locks[segmentID] = lock
		return true, "", nil
	}

	if current.TxID == lock.TxID {
		if lock.ExpiresAt.After(current.ExpiresAt) {
			s.locks[segmentID] = lock
		}
		return true, "", nil
	}

	if isBetterPriority(
		lock.LamportClock, 
		lock.CoordinatorID, 
		current.LamportClock, 
		current.CoordinatorID,
		) {

		evictedTxID := current.TxID
		s.locks[segmentID] = lock
		return true, evictedTxID, nil
	}

	return false, "", nil
}

// Get retorna uma cópia do lock atual para consulta.
func (s *LockStore) Get(segmentID string) (SegmentLock, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	lock, exists := s.locks[segmentID]
	if !exists {
		return SegmentLock{}, false
	}
	if lock.IsExpired(time.Now()) {
		delete(s.locks, segmentID)
		return SegmentLock{}, false
	}
	return lock, true
}

// Release remove o lock de um segmento caso ele pertença à transação especificada.
func (s *LockStore) Release(segmentID, txID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, exists := s.locks[segmentID]
	if !exists {
		return false
	}

	if current.TxID == txID {
		delete(s.locks, segmentID)
		return true
	}

	return false
}

// ExpireAll varre todos os locks e remove aqueles cujo TTL venceu.
func (s *LockStore) ExpireAll() []ExpiredLock {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var expired []ExpiredLock

	for segmentID, lock := range s.locks {
		if lock.IsExpired(now) {
			expired = append(expired, ExpiredLock{
				SegmentID: segmentID,
				TxID:      lock.TxID,
			})
			delete(s.locks, segmentID)
		}
	}

	return expired
}
