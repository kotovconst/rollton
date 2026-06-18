package test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/kotovconst/rollton/bot/internal/core/services"
)

func TestChatLockMap_SerializesSameID(t *testing.T) {
	m := services.NewChatLockMap()
	id := uuid.New()

	var inFlight int32
	var maxInFlight int32

	worker := func(wg *sync.WaitGroup) {
		defer wg.Done()
		m.Lock(id)
		n := atomic.AddInt32(&inFlight, 1)
		for {
			cur := atomic.LoadInt32(&maxInFlight)
			if n <= cur || atomic.CompareAndSwapInt32(&maxInFlight, cur, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		m.Unlock(id)
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go worker(&wg)
	}
	wg.Wait()

	require.Equal(t, int32(1), atomic.LoadInt32(&maxInFlight),
		"only one goroutine should ever hold the lock for the same id")
}

func TestChatLockMap_DifferentIDsRunInParallel(t *testing.T) {
	m := services.NewChatLockMap()

	var inFlight int32
	var maxInFlight int32

	worker := func(id uuid.UUID, wg *sync.WaitGroup) {
		defer wg.Done()
		m.Lock(id)
		n := atomic.AddInt32(&inFlight, 1)
		for {
			cur := atomic.LoadInt32(&maxInFlight)
			if n <= cur || atomic.CompareAndSwapInt32(&maxInFlight, cur, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		m.Unlock(id)
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go worker(uuid.New(), &wg)
	}
	wg.Wait()

	require.GreaterOrEqual(t, atomic.LoadInt32(&maxInFlight), int32(2),
		"different ids should be able to overlap")
}
