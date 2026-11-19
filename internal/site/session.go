package site

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const CookieName = "slopify_session"
const avoidCookieName = "slopify_avoid"

// GenerateTimeout bounds one model call. Background prefetch uses the same limit
// and does not follow the HTTP request context, so a click can join a job that
// outlives the response that started it.
const GenerateTimeout = 3 * time.Minute

type Store struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

type Session struct {
	mu         sync.Mutex
	id         string
	site       *Site
	pages      map[string]Page
	inflight   map[string]*routeCall
	lastHTML   string
	records    map[string][]record
	avoidFrame string
}

// routeCall is the single generation for one path. Waiters block on done.
type routeCall struct {
	done chan struct{}
	page Page
	err  error
}

func NewStore() *Store {
	return &Store{sessions: map[string]*Session{}}
}

func (st *Store) Session(w http.ResponseWriter, r *http.Request) *Session {
	if c, err := r.Cookie(CookieName); err == nil && c.Value != "" {
		st.mu.Lock()
		sess := st.sessions[c.Value]
		st.mu.Unlock()
		if sess != nil {
			return sess
		}
	}

	id := NewID()
	sess := &Session{id: id, pages: map[string]Page{}}
	if c, err := r.Cookie(avoidCookieName); err == nil {
		if _, ok := parseBrief(c.Value); ok {
			sess.avoidFrame = c.Value
		}
		http.SetCookie(w, &http.Cookie{
			Name:     avoidCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})
	}
	st.mu.Lock()
	st.sessions[id] = sess
	st.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24 * 7,
	})
	return sess
}

func (st *Store) Forget(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil && c.Value != "" {
		st.mu.Lock()
		sess := st.sessions[c.Value]
		delete(st.sessions, c.Value)
		st.mu.Unlock()
		if sess != nil {
			if value := sess.avoidValue(); value != "" {
				http.SetCookie(w, &http.Cookie{
					Name:     avoidCookieName,
					Value:    value,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
					MaxAge:   600,
				})
			}
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (s *Session) avoidValue() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.site == nil {
		return ""
	}
	return s.site.Brief
}

// Ready reports whether route can be rendered without calling the generator.
func (s *Session) Ready(route string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.site == nil {
		return false
	}
	_, ok := s.pages[route]
	return ok
}

// View returns the session's site and the page for path. A path already being
// generated is waited on, not started again. A successful page starts generation
// for every other route linked from it.
func (s *Session) View(ctx context.Context, g Generator, route string) (Site, Page, error) {
	if err := s.ensureSite(ctx, g); err != nil {
		return Site{}, Page{}, err
	}
	page, err := s.page(ctx, g, route)
	s.mu.Lock()
	var site Site
	if s.site != nil {
		site = *s.site
	}
	if err == nil {
		s.lastHTML = page.HTML
	}
	s.mu.Unlock()
	if err != nil {
		return site, Page{}, err
	}
	s.prefetch(g, site, page)
	page.HTML = armStateRefresh(page.HTML)
	return site, page, nil
}

func (s *Session) ensureSite(ctx context.Context, g Generator) error {
	s.mu.Lock()
	if s.site != nil {
		s.mu.Unlock()
		return nil
	}
	call, owned := s.claimLocked("/")
	id, avoid := s.id, s.avoidFrame
	s.mu.Unlock()
	if owned {
		go s.finishHome(g, id, avoid, call)
	}
	return waitCall(ctx, call)
}

func (s *Session) page(ctx context.Context, g Generator, route string) (Page, error) {
	s.mu.Lock()
	if page, ok := s.pages[route]; ok {
		s.mu.Unlock()
		return page, nil
	}
	if s.site == nil {
		s.mu.Unlock()
		return Page{}, errors.New("missing site")
	}
	site := *s.site
	source := s.lastHTML
	if source == "" {
		source = site.HomeHTML
	}
	call, owned := s.claimLocked(route)
	s.mu.Unlock()
	if owned {
		go s.finishRoute(g, site, source, route, call)
	}
	if err := waitCall(ctx, call); err != nil {
		return Page{}, err
	}
	return call.page, nil
}

func (s *Session) claimLocked(route string) (*routeCall, bool) {
	if s.inflight == nil {
		s.inflight = map[string]*routeCall{}
	}
	if call := s.inflight[route]; call != nil {
		return call, false
	}
	call := &routeCall{done: make(chan struct{})}
	s.inflight[route] = call
	return call, true
}

func waitCall(ctx context.Context, call *routeCall) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-call.done:
		return call.err
	}
}

func (s *Session) finishHome(g Generator, id, avoid string, call *routeCall) {
	ctx, cancel := context.WithTimeout(context.Background(), GenerateTimeout)
	defer cancel()
	site, home, err := GenerateHome(ctx, g, id, avoid)
	s.mu.Lock()
	if err == nil && s.site == nil {
		s.site = &site
		s.pages["/"] = home
	}
	if err == nil {
		call.page = s.pages["/"]
	}
	call.err = err
	delete(s.inflight, "/")
	close(call.done)
	s.mu.Unlock()
}

func (s *Session) finishRoute(g Generator, site Site, sourceHTML, route string, call *routeCall) {
	ctx, cancel := context.WithTimeout(context.Background(), GenerateTimeout)
	defer cancel()
	page, err := GenerateRoute(ctx, g, site, sourceHTML, route)
	s.mu.Lock()
	if err == nil {
		page.Path = route
		s.pages[route] = page
		call.page = page
	}
	call.err = err
	delete(s.inflight, route)
	close(call.done)
	s.mu.Unlock()
	if err != nil {
		slog.Error("generate route", "path", route, "err", err)
	}
}

// prefetch starts one generation per linked route that is not cached or already running.
func (s *Session) prefetch(g Generator, site Site, page Page) {
	for _, route := range linkedRoutes(site.Compose(page.HTML)) {
		if route == page.Path {
			continue
		}
		s.mu.Lock()
		if s.site == nil {
			s.mu.Unlock()
			return
		}
		if _, ok := s.pages[route]; ok {
			s.mu.Unlock()
			continue
		}
		if s.inflight[route] != nil {
			s.mu.Unlock()
			continue
		}
		call, _ := s.claimLocked(route)
		snapshot := *s.site
		source := page.HTML
		s.mu.Unlock()
		go s.finishRoute(g, snapshot, source, route, call)
	}
}

func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func ErrorText(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "The generator timed out."
	}
	return clip(err.Error(), 240)
}
