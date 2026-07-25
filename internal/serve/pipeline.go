package serve

import (
	"context"
	"sync"
	"sync/atomic"
)

// pipeline serializes viewer renders so concurrent GET / requests never run N
// redundant renders. It is a single-flight coalescer with a specific freshness
// guarantee: at most one render runs at a time ("in flight") and at most one
// render is "queued" behind it. A request that arrives while a render is in
// flight does NOT reuse that render's result (it may predate a comment mutation
// the request should see); it joins the queued render, which starts only after
// the in-flight one finishes and therefore reads the latest claims from disk.
// Every request that arrives during a given in-flight render shares the single
// queued render's result. So N simultaneous requests cost at most two renders,
// not N — the property the concurrency test asserts via runCount.
type pipeline struct {
	// fn does the actual work (load claims -> build -> render); it returns the
	// rendered bytes or the first error. It is called with no lock held.
	fn func() ([]byte, error)

	mu       sync.Mutex
	inflight *batch // the render currently executing, or nil if idle
	queued   *batch // the single render waiting to run next, or nil

	runs atomic.Int64
}

// batch is one render whose result is shared by every caller waiting on it.
// done is closed exactly once, when res is populated.
type batch struct {
	done chan struct{}
	res  renderResult
}

type renderResult struct {
	html []byte
	err  error
}

func newPipeline(fn func() ([]byte, error)) *pipeline {
	return &pipeline{fn: fn}
}

// get returns the result of a render that started at or after this call —
// either by launching a fresh render (when the pipeline is idle) or by joining
// the one render queued behind the in-flight one. It blocks until that render
// completes or ctx is cancelled (the request went away), whichever comes first.
func (p *pipeline) get(ctx context.Context) ([]byte, error) {
	p.mu.Lock()
	var b *batch
	if p.inflight == nil {
		// Idle: start a render for this caller and drive the queue from a
		// dedicated goroutine so waiters (including this one) only ever block
		// on their batch, never on running fn themselves.
		b = &batch{done: make(chan struct{})}
		p.inflight = b
		go p.drain(b)
	} else {
		// A render is in flight; its result may be stale for us, so wait for
		// the next one. All callers arriving now share that single queued batch.
		if p.queued == nil {
			p.queued = &batch{done: make(chan struct{})}
		}
		b = p.queued
	}
	p.mu.Unlock()

	select {
	case <-b.done:
		return b.res.html, b.res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// drain runs the given batch, then keeps promoting a queued batch into flight
// and running it until none remains. A single goroutine owns the whole chain,
// so renders never overlap and each queued render observes the freshest claims.
func (p *pipeline) drain(b *batch) {
	for b != nil {
		p.runs.Add(1)
		html, err := p.fn()

		p.mu.Lock()
		b.res = renderResult{html: html, err: err}
		close(b.done)
		// Promote whatever queued up while this render ran into the next slot.
		p.inflight = p.queued
		p.queued = nil
		b = p.inflight
		p.mu.Unlock()
	}
}

// refresh triggers a background render so a change the watcher detected is
// already reflected in memory before the client's SSE-driven re-fetch of GET /
// arrives. It reuses get's single-flight machinery via a fresh context, so a
// refresh that overlaps in-flight GET / requests COALESCES into the same render
// instead of adding a redundant one, and it never blocks the watcher's poll
// loop. The render writes nothing to disk (renderViewer is in-memory only), so
// a refresh can never itself re-trigger the watcher.
func (p *pipeline) refresh() {
	go func() { _, _ = p.get(context.Background()) }()
}

// runCount reports how many renders have executed (for the single-flight test).
func (p *pipeline) runCount() int64 { return p.runs.Load() }
