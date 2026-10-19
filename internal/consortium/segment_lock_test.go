package consortium

import (
	"testing"
	"time"
)

func TestLockStoreTryAcquireSuccess(t *testing.T) {
	store := NewLockStore(10 * time.Second)

	lock := SegmentLock{
		TxID:          "tx-1",
		CoordinatorID: "empresa-a",
		LamportClock:  10,
	}

	acquired, evicted, err := store.TryAcquire("seg-1", lock)
	if err != nil || !acquired || evicted != "" {
		t.Fatalf("falha ao adquirir lock inicial: acquired=%v, evicted=%s, err=%v", acquired, evicted, err)
	}

	current, exists := store.Get("seg-1")
	if !exists || current.TxID != "tx-1" {
		t.Fatalf("lock gravado incorreto: %+v", current)
	}
}

func TestLockStoreIdempotency(t *testing.T) {
	store := NewLockStore(10 * time.Second)

	lock := SegmentLock{
		TxID:          "tx-1",
		CoordinatorID: "empresa-a",
		LamportClock:  10,
	}

	store.TryAcquire("seg-1", lock)
	// Repetição do mesmo txID
	acquired, evicted, err := store.TryAcquire("seg-1", lock)
	if err != nil || !acquired || evicted != "" {
		t.Fatalf("idempotência falhou: acquired=%v, evicted=%s, err=%v", acquired, evicted, err)
	}
}

func TestLockStoreConflictLowerLamportWins(t *testing.T) {
	store := NewLockStore(10 * time.Second)

	// Primeiro chega um pedido com Lamport 20
	lock20 := SegmentLock{
		TxID:          "tx-loser",
		CoordinatorID: "empresa-b",
		LamportClock:  20,
	}
	acquired, _, _ := store.TryAcquire("seg-1", lock20)
	if !acquired {
		t.Fatal("lock20 deveria ter sido adquirido inicialmente")
	}

	// Agora chega um pedido com Lamport 10 (logicamente anterior)
	lock10 := SegmentLock{
		TxID:          "tx-winner",
		CoordinatorID: "empresa-c",
		LamportClock:  10,
	}
	acquired, evicted, err := store.TryAcquire("seg-1", lock10)
	if err != nil || !acquired {
		t.Fatalf("lock10 deveria vencer: acquired=%v, err=%v", acquired, err)
	}
	if evicted != "tx-loser" {
		t.Fatalf("esperado desalojamento de tx-loser, obtido %s", evicted)
	}

	current, _ := store.Get("seg-1")
	if current.TxID != "tx-winner" {
		t.Fatalf("novo lock ativo deveria ser tx-winner, obtido %s", current.TxID)
	}

	// Se agora chegar um pedido com Lamport 30 (logicamente posterior), deve perder
	lock30 := SegmentLock{
		TxID:          "tx-late",
		CoordinatorID: "empresa-a",
		LamportClock:  30,
	}
	acquired, _, _ = store.TryAcquire("seg-1", lock30)
	if acquired {
		t.Fatal("lock30 não deveria vencer lock ativo de Lamport 10")
	}
}

func TestLockStoreTiebreakerByCompanyID(t *testing.T) {
	store := NewLockStore(10 * time.Second)

	// Dois pedidos com exatamente o mesmo Lamport (15), vindos de empresas diferentes
	lockB := SegmentLock{
		TxID:          "tx-empresa-b",
		CoordinatorID: "empresa-b",
		LamportClock:  15,
	}
	store.TryAcquire("seg-1", lockB)

	// "empresa-a" < "empresa-b" lexicograficamente: empresa-a deve vencer o empate
	lockA := SegmentLock{
		TxID:          "tx-empresa-a",
		CoordinatorID: "empresa-a",
		LamportClock:  15,
	}
	acquired, evicted, _ := store.TryAcquire("seg-1", lockA)
	if !acquired || evicted != "tx-empresa-b" {
		t.Fatalf("empresa-a deveria vencer o empate: acquired=%v, evicted=%s", acquired, evicted)
	}

	// Se chegar "empresa-c" com mesmo Lamport (15), perde para "empresa-a"
	lockC := SegmentLock{
		TxID:          "tx-empresa-c",
		CoordinatorID: "empresa-c",
		LamportClock:  15,
	}
	acquired, _, _ = store.TryAcquire("seg-1", lockC)
	if acquired {
		t.Fatal("empresa-c não deveria vencer empresa-a em empate de Lamport")
	}
}

func TestLockStoreRelease(t *testing.T) {
	store := NewLockStore(10 * time.Second)

	lock := SegmentLock{
		TxID:          "tx-1",
		CoordinatorID: "empresa-a",
		LamportClock:  10,
	}
	store.TryAcquire("seg-1", lock)

	// Release com txID errado não deve liberar
	if store.Release("seg-1", "tx-errado") {
		t.Fatal("não deveria liberar lock com txID diferente")
	}

	// Release com txID correto libera
	if !store.Release("seg-1", "tx-1") {
		t.Fatal("deveria liberar lock com txID correto")
	}

	_, exists := store.Get("seg-1")
	if exists {
		t.Fatal("lock ainda existe após release")
	}
}

func TestLockStoreExpireAll(t *testing.T) {
	store := NewLockStore(50 * time.Millisecond)

	lock := SegmentLock{
		TxID:          "tx-expire",
		CoordinatorID: "empresa-a",
		LamportClock:  10,
		ExpiresAt:     time.Now().Add(10 * time.Millisecond),
	}
	store.TryAcquire("seg-1", lock)

	time.Sleep(30 * time.Millisecond)

	expired := store.ExpireAll()
	if len(expired) != 1 || expired[0].TxID != "tx-expire" {
		t.Fatalf("expiração falhou: %+v", expired)
	}

	_, exists := store.Get("seg-1")
	if exists {
		t.Fatal("lock expirado deveria ter sido removido")
	}
}
