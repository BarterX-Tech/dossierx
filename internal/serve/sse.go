package serve

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// keepAliveInterval is how often an otherwise-idle /api/events stream emits a
// comment frame. It keeps intermediaries (and a browser's own idle timers) from
// dropping a connection that is simply waiting for the next change; the frame is
// an SSE comment (": ...\n\n"), never a "changed" event, so a client ignores it.
const keepAliveInterval = 20 * time.Second

// hub is the server-sent-events fan-out behind live reload. Each connected
// /api/events handler holds one subscription: a capacity-1 channel carrying a
// bare "something changed, re-fetch" signal. The watcher calls broadcast on
// every debounced change; broadcast does a NON-BLOCKING send to each channel
// and drops when the channel is already full, because the signal is idempotent
// (one pending "changed" already tells the client to re-fetch, so a second is
// redundant). A slow or never-reading subscriber therefore can never stall the
// broadcaster or, transitively, a writer whose file change triggered it.
type hub struct {
	mu   sync.Mutex
	subs map[chan struct{}]struct{}
}

func newHub() *hub {
	return &hub{subs: make(map[chan struct{}]struct{})}
}

// Subscribe registers a new subscriber and returns its signal channel plus an
// idempotent unsubscribe. The /api/events handler MUST defer the unsubscribe so
// the hub never retains a channel whose handler has returned (the leak a plain
// -race run would not catch — a test asserts hub size directly instead). The
// channel is never closed by the hub: unsubscribe only removes it from the
// fan-out set, so a broadcast racing an unsubscribe (both take mu) can never
// send on a closed channel.
func (h *hub) Subscribe() (events <-chan struct{}, cancel func()) {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	unsub := func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subs, ch)
			h.mu.Unlock()
		})
	}
	return ch, unsub
}

// broadcast signals every current subscriber that the claims changed. The send
// is non-blocking: a subscriber whose channel already holds a pending signal is
// skipped, since that pending signal will drive a re-fetch of the freshest state
// anyway. Holding mu across the loop serializes broadcast against
// Subscribe/unsubscribe, so no send ever targets an already-removed subscriber.
func (h *hub) broadcast() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- struct{}{}:
		default:
			// Full: a "changed" is already pending for this subscriber.
		}
	}
}

// size reports the number of live subscribers. It exists for tests to prove a
// disconnect drops the subscription (a leaked channel is invisible to -race);
// production code has no reason to call it.
func (h *hub) size() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

// handleEvents is GET /api/events: a Server-Sent-Events stream that emits a bare
// "changed" event whenever a claim file changes on disk — an external edit or a
// comment mutation, both observed identically by the watcher. The browser viewer
// listens for it and re-fetches the render and /api/status; the event carries no
// payload because the client always re-reads the authoritative state.
//
// Lifecycle: subscribe, then ALWAYS defer unsub so the hub drops this subscriber
// the instant the handler returns; loop selecting on the request context (client
// disconnected -> return -> unsub), the server's closing channel (graceful
// shutdown -> return so Shutdown does not wait out the grace period on a live
// stream), the signal channel (-> emit a "changed" frame), and a keep-alive
// ticker (-> emit a comment frame so an idle stream stays open). The http.Server
// runs with WriteTimeout 0 precisely so this long-lived response is never killed
// mid-wait.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch, unsub := s.hub.Subscribe()
	defer unsub()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	// An initial comment frame opens the stream immediately, so the client (and
	// any buffering intermediary) sees bytes without waiting for the first
	// change, and the subscription is already registered by the time the client's
	// response headers arrive.
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	keepAlive := time.NewTicker(keepAliveInterval)
	defer keepAlive.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.closing:
			return
		case <-ch:
			fmt.Fprint(w, "event: changed\ndata: 1\n\n")
			flusher.Flush()
		case <-keepAlive.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}
