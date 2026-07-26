package serve

import (
	"testing"
	"time"
)

// A live subscriber receives the signal a broadcast sends.
func TestHub_SubscribeAndBroadcastDelivers(t *testing.T) {
	h := newHub()
	ch, unsub := h.Subscribe()
	defer unsub()

	if got := h.size(); got != 1 {
		t.Fatalf("hub size after Subscribe = %d, want 1", got)
	}
	h.broadcast()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("broadcast did not deliver a signal to a live subscriber")
	}
}

// Unsubscribe removes the subscriber and is idempotent — a second call is a
// no-op, never a panic or a negative count. This is the exact leak -race cannot
// see, so the hub size is asserted directly.
func TestHub_UnsubscribeRemovesSubscriber(t *testing.T) {
	h := newHub()
	_, unsub := h.Subscribe()
	if got := h.size(); got != 1 {
		t.Fatalf("hub size = %d, want 1", got)
	}
	unsub()
	if got := h.size(); got != 0 {
		t.Fatalf("hub size after unsub = %d, want 0", got)
	}
	unsub() // idempotent
	if got := h.size(); got != 0 {
		t.Fatalf("hub size after double unsub = %d, want 0", got)
	}
}

// A never-read, already-full subscriber channel must not block the broadcaster.
// This is the guarantee that a stalled SSE client can never delay the writer
// whose file change (transitively) triggers a broadcast.
func TestHub_BroadcastNonBlockingWhenSubscriberFull(t *testing.T) {
	h := newHub()
	_, unsub := h.Subscribe() // deliberately never read this subscriber's channel
	defer unsub()

	h.broadcast() // fills the capacity-1 channel

	done := make(chan struct{})
	go func() {
		h.broadcast() // must drop, not block
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast blocked on a full, never-read subscriber channel")
	}
}

// Repeated broadcasts with no intervening read collapse to a single pending
// signal (capacity-1, idempotent): the reader sees exactly one, not three.
func TestHub_BroadcastCoalescesToOnePending(t *testing.T) {
	h := newHub()
	ch, unsub := h.Subscribe()
	defer unsub()

	h.broadcast()
	h.broadcast()
	h.broadcast()

	<-ch // one delivered
	select {
	case <-ch:
		t.Fatal("a second signal was queued; broadcasts must coalesce to one pending")
	case <-time.After(50 * time.Millisecond):
	}
}
