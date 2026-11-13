package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	shirei "go.hasen.dev/shirei"
)

// gate is a channel that can be released exactly once; tests close it to let a
// fake provider finish, and cleanup releases it too so a failing assertion
// cannot leave httptest.Server.Close waiting on a stuck handler.
type gate struct {
	once sync.Once
	ch   chan struct{}
}

func newGate() *gate { return &gate{ch: make(chan struct{})} }

func (g *gate) release() { g.once.Do(func() { close(g.ch) }) }

// gatedProvider is a handler that reports when it was reached, then blocks
// until its gate is released — a data provider that never answers quickly.
func gatedProvider(seen chan<- struct{}, g *gate, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case seen <- struct{}{}:
		default:
		}
		select {
		case <-g.ch:
		case <-r.Context().Done():
			return
		}
		fmt.Fprint(w, body)
	}
}

type slowProviders struct {
	srv       *httptest.Server
	tle       *gate
	transpond *gate
	tleSeen   chan struct{}
	xpSeen    chan struct{}
}

// newSlowProviders starts a fake TLE provider and a fake transponder provider
// that both stay silent until their gate is released.
func newSlowProviders(t *testing.T, tleBody, xpBody string) *slowProviders {
	t.Helper()
	p := &slowProviders{
		tle:       newGate(),
		transpond: newGate(),
		tleSeen:   make(chan struct{}, 16),
		xpSeen:    make(chan struct{}, 16),
	}
	mux := http.NewServeMux()
	mux.Handle("/tle", gatedProvider(p.tleSeen, p.tle, tleBody))
	mux.Handle("/xp", gatedProvider(p.xpSeen, p.transpond, xpBody))
	p.srv = httptest.NewServer(mux)
	t.Cleanup(func() {
		p.tle.release()
		p.transpond.release()
		p.srv.Close()
	})
	return p
}

// prepareFetchTest installs the fake providers and clears the fetch guards so a
// refresh starts from a clean state.
func prepareFetchTest(t *testing.T, p *slowProviders) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	loadDemoData()
	st.fetching = false
	st.fetchErrs = nil
	st.fetchNotice = ""
	st.lastFetch = time.Time{}
	st.lastAttempt = time.Time{}
	st.fromCache = false
	st.view = viewMain
	st.cfg.TleSources = []string{p.srv.URL + "/tle"}
	st.cfg.TransponderSources = []string{p.srv.URL + "/xp"}
}

func waitDone(t *testing.T, ch <-chan struct{}, timeout time.Duration, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatal(msg)
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal(msg)
}

// assertFramesRender renders a handful of frames and fails if any of them
// blocks. A UI that waits on a slow provider cannot finish the frame before the
// test releases the provider gate.
func assertFramesRender(t *testing.T, frames int) {
	t.Helper()
	for i := 0; i < frames; i++ {
		done := make(chan struct{})
		go func() {
			defer close(done)
			shirei.RenderToImage(900, 620, RootView)
		}()
		waitDone(t, done, 2*time.Second, "a frame blocked while the data providers were slow")
	}
}

// TestRefreshFromFrameDoesNotBlock covers the entry points that start a fetch
// from inside a frame (Refresh button, Preferences Save): they must never take
// the frame lock, or the whole app deadlocks. While the fake providers hang, the
// UI must keep rendering frames and must settle once they answer.
func TestRefreshFromFrameDoesNotBlock(t *testing.T) {
	l1, l2 := cloneTLE(demoISS1, demoISS2, 90007, 120, 60)
	p := newSlowProviders(t,
		"SLOW-SAT\n"+l1+"\n"+l2+"\n",
		"SLOW-SAT;90007;145.900;437.800;;FM;N0CALL;active\n")
	prepareFetchTest(t, p)

	// requestRefresh from a frame handler is the UI-thread path; it used to
	// call startFetch synchronously and deadlock on the frame lock.
	var once sync.Once
	frameDone := make(chan struct{})
	go func() {
		defer close(frameDone)
		shirei.RenderToImage(400, 300, func() { once.Do(requestRefresh) })
	}()
	waitDone(t, frameDone, 5*time.Second, "requestRefresh from a frame handler deadlocked the UI thread")
	waitDone(t, p.tleSeen, 5*time.Second, "the TLE provider was never requested")

	// The TLE download is in flight (the handler is blocked): frames must not.
	assertFramesRender(t, 5)

	p.tle.release()
	waitDone(t, p.xpSeen, 5*time.Second, "the transponder provider was never requested")

	// Same for the transponder download.
	assertFramesRender(t, 5)

	p.transpond.release()
	waitFor(t, 5*time.Second, func() bool {
		done := false
		shirei.WithFrameLock(func() {
			sat := st.sats[90007]
			done = !st.fetching && sat != nil && len(sat.Transponders) == 1
		})
		return done
	}, "refresh did not finish with the fetched satellite and transponder")
}
