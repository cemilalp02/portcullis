package engine

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestByteBudgetBoundsBytesInFlight(t *testing.T) {
	const total = 100
	b := newByteBudget(total)
	var inFlight, peak atomic.Int64
	var wg sync.WaitGroup
	for i := range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n := int64(i%total + 1)
			b.acquire(n)
			cur := inFlight.Add(n)
			for {
				p := peak.Load()
				if cur <= p || peak.CompareAndSwap(p, cur) {
					break
				}
			}
			inFlight.Add(-n)
			b.release(n)
		}()
	}
	wg.Wait()
	if p := peak.Load(); p > total {
		t.Errorf("%d bytes in flight, budget %d", p, total)
	}
	if b.free != total {
		t.Errorf("budget not restored: %d of %d free", b.free, total)
	}
}
