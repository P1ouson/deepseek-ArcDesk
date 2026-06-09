package control

import (
	"context"
	"sync"
	"testing"
	"time"

	"arcdesk/internal/agent"
	"arcdesk/internal/event"
	"arcdesk/internal/i18n"
)

type blockingRunner struct {
	once    sync.Once
	started chan struct{}
	release chan struct{}
}

func (r blockingRunner) Run(ctx context.Context, _ string) error {
	r.once.Do(func() { close(r.started) })
	select {
	case <-r.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestSendWhileRunningEmitsBusyNotice(t *testing.T) {
	sink, done, events := collectSink()
	runner := blockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	ctrl := New(Options{Runner: runner, Sink: sink})

	ctrl.Send("first")
	<-runner.started

	ctrl.Send("second")

	var sawBusy bool
	for _, e := range *events {
		if e.Kind == event.Notice && e.Text == i18n.M.AgentBusy {
			sawBusy = true
		}
	}
	if !sawBusy {
		t.Fatalf("second Send should emit AgentBusy notice, events=%+v", *events)
	}

	close(runner.release)
	waitForDone(t, done)

	var turnDones int
	for _, e := range *events {
		if e.Kind == event.TurnDone {
			turnDones++
		}
	}
	if turnDones != 1 {
		t.Fatalf("want exactly one TurnDone, got %d", turnDones)
	}
}

func TestConcurrentSendGuardIsRaceSafe(t *testing.T) {
	sink, done, events := collectSink()
	runner := blockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	ctrl := New(Options{Runner: runner, Sink: sink})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ctrl.Send("msg")
		}(i)
	}
	wg.Wait()

	<-runner.started
	close(runner.release)
	waitForDone(t, done)

	busy := 0
	turnDones := 0
	for _, e := range *events {
		switch e.Kind {
		case event.Notice:
			if e.Text == i18n.M.AgentBusy {
				busy++
			}
		case event.TurnDone:
			turnDones++
		}
	}
	if turnDones != 1 {
		t.Fatalf("want one TurnDone, got %d", turnDones)
	}
	if busy < 1 {
		t.Fatalf("want at least one busy notice, got %d", busy)
	}
	if busy > 7 {
		t.Fatalf("want at most 7 busy notices, got %d", busy)
	}
}

func TestSendAfterTurnCompletes(t *testing.T) {
	sink, done, events := collectSink()
	sess := agent.NewSession("sys")
	ctrl := New(Options{Runner: appendingRunner{session: sess}, Sink: sink})

	ctrl.Send("first")
	waitForDone(t, done)

	ctrl.Send("second")
	select {
	case e := <-done:
		if e.Err != nil {
			t.Fatalf("second turn failed: %v", e.Err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for second TurnDone")
	}

	busy := 0
	turnDones := 0
	for _, e := range *events {
		switch e.Kind {
		case event.Notice:
			if e.Text == i18n.M.AgentBusy {
				busy++
			}
		case event.TurnDone:
			turnDones++
		}
	}
	if busy != 0 {
		t.Fatalf("sequential sends should not emit busy, got %d notices", busy)
	}
	if turnDones != 2 {
		t.Fatalf("want two TurnDone events, got %d", turnDones)
	}
}
