package synchronization_test

import (
	"context"
	"github.com/orbs-network/orbs-network-go/synchronization"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
)

func getExpected(startTime, endTime time.Time, tickTime time.Duration) uint32 {
	duration := endTime.Sub(startTime)
	expected := uint32((duration.Seconds() * 1000) / (tickTime.Seconds() * 1000))
	return expected
}

func TestNewPeriodicalTrigger(t *testing.T) {
	p := synchronization.NewPeriodicalTrigger(context.Background(), time.Duration(5), func() {})
	require.NotNil(t, p, "failed to initialize the ticker")
	require.False(t, p.IsRunning(), "should not be running when created")
}

func TestPeriodicalTrigger_NoStartDoesNotFireFunc(t *testing.T) {
	x := 0
	p := synchronization.NewPeriodicalTrigger(context.Background(), time.Millisecond*1, func() { x++ })
	time.Sleep(time.Millisecond * 10)
	require.Equal(t, 0, x, "expected no ticks")
	p.Stop() // to hold the reference
}

func TestPeriodicalTrigger_Start(t *testing.T) {
	var x uint32
	start := time.Now()
	tickTime := 5 * time.Millisecond
	p := synchronization.NewPeriodicalTrigger(context.Background(), tickTime, func() { atomic.AddUint32(&x, 1) })
	p.Start()
	time.Sleep(time.Millisecond * 30)
	expected := getExpected(start, time.Now(), tickTime)
	require.True(t, expected/2 < atomic.LoadUint32(&x), "expected more than %d ticks, but got %d", expected/2, atomic.LoadUint32(&x))
	p.Stop()
}

func TestTriggerInternalMetrics(t *testing.T) {
	var x uint32
	start := time.Now()
	tickTime := 5 * time.Millisecond
	p := synchronization.NewPeriodicalTrigger(context.Background(), tickTime, func() { atomic.AddUint32(&x, 1) })
	p.Start()
	time.Sleep(time.Millisecond * 30)
	expected := getExpected(start, time.Now(), tickTime)
	require.True(t, expected/2 < atomic.LoadUint32(&x), "expected more than %d ticks, but got %d", expected/2, atomic.LoadUint32(&x))
	require.True(t, uint64(expected/2) < p.TimesTriggered(), "expected more than %d ticks, but got %d (metric)", expected/2, p.TimesTriggered())
	p.Stop()
}

func TestPeriodicalTrigger_Reset(t *testing.T) {
	release := make(chan struct{})
	p := synchronization.NewPeriodicalTrigger(context.Background(), time.Millisecond*1, func() { release <- struct{}{} })
	start := time.Now()
	p.Start()
	p.Reset(time.Millisecond * 2)
	require.EqualValues(t, 1, p.TimesReset(), "expected reset counter to be one")
	<-release
	duration := time.Since(start)
	require.True(t, duration.Seconds() > 0.002, "expected test to take at least 2ms, but not less, it took %f", duration.Seconds())
	p.Stop()
}

func TestPeriodicalTrigger_FireNow(t *testing.T) {
	x := 0
	p := synchronization.NewPeriodicalTrigger(context.Background(), time.Millisecond*50, func() { x++ })
	p.Start()
	time.Sleep(time.Millisecond * 25)
	p.FireNow()
	time.Sleep(time.Millisecond * 35)
	// at this point we are 50+ ms into the logic, we should see only one tick as we reset though firenow midway
	require.Equal(t, 1, x, "expected one tick for now")
	require.EqualValues(t, 0, p.TimesTriggered(), "expected to not have a timer tick trigger now, got %d ticks", p.TimesTriggered())
	require.EqualValues(t, 0, p.TimesReset(), "should not count a reset on firenow")
	require.EqualValues(t, 1, p.TimesTriggeredManually(), "we triggered manually once")
	p.Stop()
}

func TestPeriodicalTrigger_Stop(t *testing.T) {
	x := 0
	p := synchronization.NewPeriodicalTrigger(context.Background(), time.Millisecond*2, func() { x++ })
	p.Start()
	p.Stop()
	time.Sleep(3 * time.Millisecond)
	require.Equal(t, 0, x, "expected no ticks")
}

func TestPeriodicalTrigger_StopAfterTrigger(t *testing.T) {
	x := 0
	p := synchronization.NewPeriodicalTrigger(context.Background(), time.Millisecond, func() { x++ })
	p.Start()
	time.Sleep(time.Microsecond * 1100)
	p.Stop()
	xValueOnStop := x
	time.Sleep(time.Millisecond * 5)
	require.Equal(t, xValueOnStop, x, "expected one tick due to stop")
}

func TestPeriodicalTriggerStopOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	x := 0
	p := synchronization.NewPeriodicalTrigger(ctx, time.Millisecond*2, func() { x++ })
	p.Start()
	cancel()
	time.Sleep(3 * time.Millisecond)
	require.Equal(t, 0, x, "expected no ticks")
}
