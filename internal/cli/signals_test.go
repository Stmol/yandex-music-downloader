package cli

import (
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

const signalTestTimeout = time.Second

func TestInterruptSignalsRouteFirstAndSecondSIGINTAndSIGTERM(t *testing.T) {
	tests := []struct {
		name   string
		signal os.Signal
	}{
		{name: "SIGINT", signal: syscall.SIGINT},
		{name: "SIGTERM", signal: syscall.SIGTERM},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interrupts := newInterruptSignals()
			t.Cleanup(interrupts.stop)
			process, err := os.FindProcess(os.Getpid())
			if err != nil {
				t.Fatalf("find current process: %v", err)
			}

			if err := process.Signal(tt.signal); err != nil {
				t.Fatalf("send first %s: %v", tt.name, err)
			}
			waitForSignalStage(t, interrupts.first, "first")
			assertSignalStagePending(t, interrupts.force, "force")

			if err := process.Signal(tt.signal); err != nil {
				t.Fatalf("send second %s: %v", tt.name, err)
			}
			waitForSignalStage(t, interrupts.force, "force")
		})
	}
}

func TestInterruptSignalsStopIsIdempotent(t *testing.T) {
	signalCh := make(chan os.Signal, 2)
	var unregisterCalls atomic.Int32
	interrupts := routeInterruptSignals(signalCh, func() {
		unregisterCalls.Add(1)
	})

	interrupts.stop()
	interrupts.stop()
	if got := unregisterCalls.Load(); got != 1 {
		t.Fatalf("unregister calls = %d, want 1", got)
	}
}

func TestInterruptSignalsFlushKeepsRouterRegisteredAndRunning(t *testing.T) {
	signalCh := make(chan os.Signal, 1)
	var unregisterCalls atomic.Int32
	interrupts := routeInterruptSignals(signalCh, func() {
		unregisterCalls.Add(1)
	})
	t.Cleanup(interrupts.stop)

	interrupts.flush()
	if got := unregisterCalls.Load(); got != 0 {
		t.Fatalf("unregister calls after flush = %d, want 0", got)
	}

	signalCh <- syscall.SIGTERM
	waitForSignalStage(t, interrupts.first, "first signal after flush")
	assertSignalStagePending(t, interrupts.force, "force signal after flush")

	interrupts.stop()
	interrupts.stop()
	if got := unregisterCalls.Load(); got != 1 {
		t.Fatalf("unregister calls after stop = %d, want 1", got)
	}
}

func waitForSignalStage(t *testing.T, stage <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-stage:
	case <-time.After(signalTestTimeout):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func assertSignalStagePending(t *testing.T, stage <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-stage:
		t.Fatalf("%s happened too early", name)
	default:
	}
}
