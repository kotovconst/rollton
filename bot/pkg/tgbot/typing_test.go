package tgbot

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWithTyping_PassesThroughFnResult(t *testing.T) {
	sentinel := errors.New("from fn")
	var calls int32
	send := func(_ int64) error { atomic.AddInt32(&calls, 1); return nil }

	err := withTyping(123, send, func() error { return sentinel })

	require.ErrorIs(t, err, sentinel)
	require.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(1))
}

func TestWithTyping_SendsImmediately(t *testing.T) {
	var calls int32
	send := func(_ int64) error { atomic.AddInt32(&calls, 1); return nil }

	_ = withTyping(123, send, func() error {
		return nil
	})

	require.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(1))
}

func TestWithTyping_TicksDuringLongFn(t *testing.T) {
	prev := tickInterval
	tickInterval = 50 * time.Millisecond
	t.Cleanup(func() { tickInterval = prev })

	var calls int32
	send := func(_ int64) error { atomic.AddInt32(&calls, 1); return nil }

	_ = withTyping(123, send, func() error {
		time.Sleep(180 * time.Millisecond)
		return nil
	})

	require.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(3))
}

func TestWithTyping_StopsAfterFnReturns(t *testing.T) {
	prev := tickInterval
	tickInterval = 50 * time.Millisecond
	t.Cleanup(func() { tickInterval = prev })

	var calls int32
	send := func(_ int64) error { atomic.AddInt32(&calls, 1); return nil }

	require.NoError(t, withTyping(123, send, func() error { return nil }))
	before := atomic.LoadInt32(&calls)
	time.Sleep(200 * time.Millisecond)
	after := atomic.LoadInt32(&calls)

	require.Equal(t, before, after, "no additional sends after fn returned")
}

func TestWithTyping_SendErrorsAreSwallowed(t *testing.T) {
	send := func(_ int64) error { return errors.New("send failed") }

	require.NoError(t, withTyping(123, send, func() error { return nil }))
}

func TestWithTyping_PanicInFnDoesNotLeakGoroutine(t *testing.T) {
	prev := tickInterval
	tickInterval = 50 * time.Millisecond
	t.Cleanup(func() { tickInterval = prev })

	var calls int32
	send := func(_ int64) error { atomic.AddInt32(&calls, 1); return nil }

	require.Panics(t, func() {
		_ = withTyping(123, send, func() error { panic("boom") })
	})

	before := atomic.LoadInt32(&calls)
	time.Sleep(150 * time.Millisecond)
	after := atomic.LoadInt32(&calls)
	require.Equal(t, before, after, "ticker goroutine must stop when fn panics")
}
