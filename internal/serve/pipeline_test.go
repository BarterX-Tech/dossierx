package serve

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// observedContext reports when pipeline.get has selected its batch and reached
// its result wait. Its embedded Background context never cancels.
type observedContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func (c *observedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.Context.Done()
}

type pipelineTestResult struct {
	html string
	err  error
}

func TestPipeline_CoalescesConcurrentWaitersIntoOneQueuedRender(t *testing.T) {
	const waiters = 30

	started := make(chan int, 2)
	releases := []chan struct{}{make(chan struct{}, 1), make(chan struct{}, 1)}
	defer func() {
		// Unblock either render if an assertion exits the test early.
		for _, release := range releases {
			select {
			case release <- struct{}{}:
			default:
			}
		}
	}()

	var runMu sync.Mutex
	runs := 0
	p := newPipeline(func() ([]byte, error) {
		runMu.Lock()
		runs++
		run := runs
		runMu.Unlock()
		if run > len(releases) {
			return nil, fmt.Errorf("unexpected render run %d", run)
		}
		started <- run
		<-releases[run-1]
		return []byte(fmt.Sprintf("render-%d", run)), nil
	})

	first := make(chan pipelineTestResult, 1)
	go func() {
		html, err := p.get(context.Background())
		first <- pipelineTestResult{html: string(html), err: err}
	}()
	waitPipelineSignal(t, started, 1)

	queued := make(chan pipelineTestResult, waiters)
	contexts := make([]*observedContext, waiters)
	for i := range waiters {
		ctx := &observedContext{Context: context.Background(), observed: make(chan struct{})}
		contexts[i] = ctx
		go func() {
			html, err := p.get(ctx)
			queued <- pipelineTestResult{html: string(html), err: err}
		}()
	}
	// Done is evaluated only after get has selected and unlocked its batch. Once
	// every signal arrives, all callers are known to be waiting on the one queued
	// batch while render 1 remains deliberately blocked.
	for i, ctx := range contexts {
		select {
		case <-ctx.observed:
		case <-time.After(2 * time.Second):
			t.Fatalf("waiter %d did not reach its batch wait", i)
		}
	}
	select {
	case run := <-started:
		t.Fatalf("render %d overlapped blocked render 1", run)
	default:
	}

	releases[0] <- struct{}{}
	waitPipelineSignal(t, started, 2)
	releases[1] <- struct{}{}

	waitPipelineResult(t, first, "render-1")
	for range waiters {
		waitPipelineResult(t, queued, "render-2")
	}
	if got := p.runCount(); got != 2 {
		t.Fatalf("pipeline executed %d renders for one active caller plus %d queued waiters, want exactly 2", got, waiters)
	}
}

func waitPipelineSignal(t *testing.T, started <-chan int, want int) {
	t.Helper()
	select {
	case got := <-started:
		if got != want {
			t.Fatalf("render start = %d, want %d", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("render %d did not start", want)
	}
}

func waitPipelineResult(t *testing.T, results <-chan pipelineTestResult, want string) {
	t.Helper()
	select {
	case got := <-results:
		if got.err != nil {
			t.Fatalf("pipeline result error: %v", got.err)
		}
		if got.html != want {
			t.Fatalf("pipeline result = %q, want %q", got.html, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("pipeline result %q did not arrive", want)
	}
}
