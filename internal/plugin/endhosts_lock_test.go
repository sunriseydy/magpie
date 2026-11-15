package plugin

import (
	"testing"
	"time"
)

// EndHosts returns while another goroutine holds hostMu. A start() holds
// it for the whole of a host startup, and its init is bounded at two
// minutes: an update restart waiting for that here never reached
// syscall.Exec, so a supervisor's SIGTERM found nothing to end and its
// SIGKILL ended magpie instead (#6643).
//
// The lock is held for longer than the bound EndHosts may wait, so an
// EndHosts that waits on it at all is still waiting when the test gives
// up, and one that gives up on it has returned by then.
func TestEndHostsReturnsWhileHostMuHeld(t *testing.T) {
	live.Lock()
	was := live.ended
	live.ended = false
	live.Unlock()
	t.Cleanup(func() {
		live.Lock()
		live.ended = was
		live.Unlock()
	})

	// part way through a start(): hostMu taken, no host counted yet
	hostMu.Lock()
	released := make(chan struct{})
	go func() {
		time.Sleep(8 * time.Second)
		hostMu.Unlock()
		close(released)
	}()

	done := make(chan struct{})
	go func() {
		EndHosts()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(6 * time.Second):
		t.Fatal("EndHosts waited on hostMu, which a start() in flight holds for a whole host startup")
	}
	<-released
	<-done
}

// EndHosts still drops the host calls go to, so none is answered by a
// host it has already ended.
func TestEndHostsDropsCurrent(t *testing.T) {
	live.Lock()
	was := live.ended
	live.ended = false
	live.Unlock()
	t.Cleanup(func() {
		live.Lock()
		live.ended = was
		live.Unlock()
	})

	hostMu.Lock()
	prev := current
	current = &host{}
	hostMu.Unlock()
	t.Cleanup(func() {
		hostMu.Lock()
		current = prev
		hostMu.Unlock()
	})

	EndHosts()

	hostMu.Lock()
	got := current
	hostMu.Unlock()
	if got != nil {
		t.Fatal("EndHosts left current set")
	}
}
