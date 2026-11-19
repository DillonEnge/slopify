package site

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type gateGen struct {
	mu      sync.Mutex
	calls   int
	release chan struct{}
	entered chan struct{}
	once    sync.Once
}

func (g *gateGen) Complete(ctx context.Context, home bool, _, input string) (string, error) {
	g.mu.Lock()
	g.calls++
	g.mu.Unlock()
	if home {
		theme := "light"
		const mark = "color: "
		if i := strings.Index(input, mark); i >= 0 {
			theme = input[i+len(mark):]
			if n := strings.IndexByte(theme, '\n'); n >= 0 {
				theme = theme[:n]
			}
			theme = strings.TrimSpace(theme)
		}
		return `<header data-theme="` + theme + `"><h1>Gate</h1><a href="/hold">Hold</a></header><main id="page"><h1>Home</h1></main>`, nil
	}
	g.once.Do(func() { close(g.entered) })
	select {
	case <-g.release:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return `<main><h1>Hold</h1></main>`, nil
}

func (g *gateGen) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls
}

func TestClickJoinsPrefetch(t *testing.T) {
	g := &gateGen{release: make(chan struct{}), entered: make(chan struct{})}
	rr := httptest.NewRecorder()
	sess := NewStore().Session(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	ctx := context.Background()
	if _, _, err := sess.View(ctx, g, "/"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-g.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("prefetch did not start")
	}
	done := make(chan error, 1)
	go func() {
		_, _, err := sess.View(ctx, g, "/hold")
		done <- err
	}()
	deadline := time.Now().Add(time.Second)
	for g.count() != 2 {
		if time.Now().After(deadline) {
			t.Fatalf("calls = %d", g.count())
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	if g.count() != 2 {
		t.Fatalf("second generation started: %d", g.count())
	}
	close(g.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if g.count() != 2 {
		t.Fatalf("calls = %d", g.count())
	}
	if !sess.Ready("/hold") {
		t.Fatal("route was not cached")
	}
}
