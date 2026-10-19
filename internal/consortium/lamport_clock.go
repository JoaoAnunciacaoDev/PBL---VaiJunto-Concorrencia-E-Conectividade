package consortium

import "sync/atomic"

// LamportClock implementa um relógio lógico de Lamport thread-safe.
// Em eventos locais, o relógio é incrementado via Tick().
// Ao receber uma mensagem remota com timestamp R, o relógio é atualizado
// atomicamente para max(local, R) + 1 via Update(R).
type LamportClock struct {
	value atomic.Uint64
}

// NewLamportClock inicializa um relógio com valor inicial.
func NewLamportClock(initial uint64) *LamportClock {
	clock := &LamportClock{}
	clock.value.Store(initial)
	return clock
}

// Tick incrementa o clock local e retorna o novo valor.
func (c *LamportClock) Tick() uint64 {
	return c.value.Add(1)
}

// Update sincroniza o clock com um timestamp remoto recebido,
// garantindo a propriedade de causalidade de Lamport:
// L(e) = max(L_local, R) + 1.
func (c *LamportClock) Update(received uint64) uint64 {
	for {
		current := c.value.Load()
		next := current
		if received > next {
			next = received
		}
		next++
		if c.value.CompareAndSwap(current, next) {
			return next
		}
	}
}

// Value retorna o valor atual do clock sem incrementá-lo.
func (c *LamportClock) Value() uint64 {
	return c.value.Load()
}
