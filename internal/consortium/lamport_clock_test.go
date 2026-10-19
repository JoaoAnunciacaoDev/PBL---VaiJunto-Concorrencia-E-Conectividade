package consortium

import (
	"sync"
	"testing"
)

func TestLamportClockTick(t *testing.T) {
	clock := NewLamportClock(0)

	if v := clock.Tick(); v != 1 {
		t.Fatalf("esperado tick 1, obtido %d", v)
	}
	if v := clock.Tick(); v != 2 {
		t.Fatalf("esperado tick 2, obtido %d", v)
	}
	if v := clock.Value(); v != 2 {
		t.Fatalf("esperado value 2, obtido %d", v)
	}
}

func TestLamportClockUpdate(t *testing.T) {
	clock := NewLamportClock(10)

	// Recebe timestamp menor que o local (10) -> avança para 11
	if v := clock.Update(5); v != 11 {
		t.Fatalf("esperado update para 11, obtido %d", v)
	}

	// Recebe timestamp maior que o local (11) -> max(11, 20) + 1 = 21
	if v := clock.Update(20); v != 21 {
		t.Fatalf("esperado update para 21, obtido %d", v)
	}
}

func TestLamportClockConcurrent(t *testing.T) {
	clock := NewLamportClock(0)
	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				clock.Tick()
			}
		}()
		go func(id uint64) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				clock.Update(id * 10)
			}
		}(uint64(i))
	}

	wg.Wait()

	if clock.Value() == 0 {
		t.Fatal("clock deveria ter valor maior que 0 após operações concorrentes")
	}
}
