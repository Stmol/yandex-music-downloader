package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type interruptSignals struct {
	first <-chan struct{}
	force <-chan struct{}
	flush func()
	stop  func()
}

type interruptSignalCommand struct {
	stop bool
	done chan struct{}
}

func newInterruptSignals() interruptSignals {
	signalCh := make(chan os.Signal, 2)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	return routeInterruptSignals(signalCh, func() {
		signal.Stop(signalCh)
	})
}

func routeInterruptSignals(signalCh <-chan os.Signal, unregister func()) interruptSignals {
	return routeInterruptSignalsWithBeforeRoute(signalCh, unregister, nil)
}

func routeInterruptSignalsWithBeforeRoute(
	signalCh <-chan os.Signal,
	unregister func(),
	beforeRoute func(),
) interruptSignals {
	first := make(chan struct{})
	force := make(chan struct{})
	commands := make(chan interruptSignalCommand)
	stopped := make(chan struct{})

	go func() {
		defer close(stopped)

		stage := 0
		routeSignal := func() {
			if beforeRoute != nil {
				beforeRoute()
			}
			switch stage {
			case 0:
				close(first)
				stage = 1
			case 1:
				close(force)
				stage = 2
			}
		}
		signals := signalCh
		drainSignals := func() {
			for signals != nil {
				select {
				case _, ok := <-signals:
					if !ok {
						signals = nil
						return
					}
					routeSignal()
				default:
					return
				}
			}
		}

		for {
			select {
			case _, ok := <-signals:
				if !ok {
					signals = nil
					continue
				}
				routeSignal()
			case command := <-commands:
				drainSignals()
				close(command.done)
				if command.stop {
					return
				}
			}
		}
	}()

	var lifecycleMu sync.Mutex
	controllerStopped := false
	sendCommand := func(stop bool) {
		done := make(chan struct{})
		select {
		case commands <- interruptSignalCommand{stop: stop, done: done}:
			select {
			case <-done:
			case <-stopped:
			}
		case <-stopped:
		}
	}
	flush := func() {
		lifecycleMu.Lock()
		defer lifecycleMu.Unlock()
		if controllerStopped {
			return
		}
		sendCommand(false)
	}
	stop := func() {
		lifecycleMu.Lock()
		defer lifecycleMu.Unlock()
		if controllerStopped {
			return
		}
		if unregister != nil {
			unregister()
		}
		sendCommand(true)
		<-stopped
		controllerStopped = true
	}

	return interruptSignals{
		first: first,
		force: force,
		flush: flush,
		stop:  stop,
	}
}
