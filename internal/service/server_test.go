package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DillonEnge/slopify/internal/site"
)

type fakeGen struct {
	mu    sync.Mutex
	calls []string
	homes []bool
}

const homeFrame = `<style>header.mast{font-size:9rem} main.deck{display:grid}</style>` +
	`<header class="mast"><h1>Northwind Ferry</h1><nav><a href="/">Home</a><a href="/sailings">Sailings</a></nav></header>` +
	`<main id="page" class="deck"><h1>Today's crossings</h1><p>Two boats are running.</p></main>` +
	`<footer><a href="/new-site" hx-boost="false">New site</a></footer>`

func (f *fakeGen) Complete(_ context.Context, home bool, _, input string) (string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, input)
	f.homes = append(f.homes, home)
	f.mu.Unlock()

	if home {
		frame := homeFrame
		if theme := briefTheme(input); theme != "" {
			frame = strings.Replace(frame, `<header class="mast">`, `<header class="mast" data-theme="`+theme+`">`, 1)
		}
		return frame, nil
	}
	path := "page"
	for _, line := range strings.Split(input, "\n") {
		if strings.HasPrefix(line, "path: ") {
			path = strings.TrimPrefix(line, "path: ")
		}
	}
	return `<main class="nope"><h1>Sailings</h1><p>Interpreted ` + path + `</p><a href="/pier/2">Noon</a></main>`, nil
}

func (f *fakeGen) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeGen) homeAt(i int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.homes[i]
}

func (f *fakeGen) callAt(i int) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[i]
}

func cookieNamed(rr *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rr.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func cookieHeader(rr *httptest.ResponseRecorder) string {
	if c := cookieNamed(rr, site.CookieName); c != nil {
		return c.Name + "=" + c.Value
	}
	return ""
}

func TestHealthAndStaticSkipGenerator(t *testing.T) {
	gen := &fakeGen{}
	h := newServer(gen).Handler()

	health := httptest.NewRecorder()
	h.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/health", nil))
	if health.Code != http.StatusNoContent || health.Body.Len() != 0 {
		t.Fatalf("health = %d %q", health.Code, health.Body.String())
	}

	css := httptest.NewRecorder()
	h.ServeHTTP(css, httptest.NewRequest(http.MethodGet, "/static/output.css", nil))
	if css.Code != http.StatusOK {
		t.Fatalf("css = %d", css.Code)
	}
	if gen.count() != 0 {
		t.Fatalf("generator calls = %d", gen.count())
	}
}

func TestSessionSiteAndRouteSwap(t *testing.T) {
	gen := &fakeGen{}
	h := newServer(gen).Handler()

	home := httptest.NewRecorder()
	h.ServeHTTP(home, httptest.NewRequest(http.MethodGet, "/", nil))
	if home.Code != http.StatusOK {
		t.Fatalf("home status = %d", home.Code)
	}
	body := home.Body.String()
	for _, want := range []string{
		"id=\"boot\"",
		"hx-get=\"/\"",
		"hx-trigger=\"load\"",
		"hx-target=\"this\"",
		"hx-swap=\"outerHTML\"",
		"waiting queue",
		"High load",
		"POSITION",
		"HOLDING",
		"htmx.org@2.0.2",
		"/static/output.css",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("loader missing %q\n%s", want, body)
		}
	}
	if strings.Contains(body, "Northwind Ferry") {
		t.Fatal("loader waited for generation")
	}
	cookie := cookieHeader(home)
	if !strings.Contains(cookie, site.CookieName+"=") {
		t.Fatalf("cookie = %q", cookie)
	}
	if gen.count() != 0 {
		t.Fatalf("loader calls = %d", gen.count())
	}

	boot := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("HX-Request", "true")
	h.ServeHTTP(boot, req)
	if boot.Code != http.StatusOK {
		t.Fatalf("boot status = %d", boot.Code)
	}
	shell := boot.Body.String()
	for _, want := range []string{
		"Northwind Ferry",
		"Today&#39;s crossings",
		`class="mast"`,
		"data-theme=",
		"/static/output.css",
		"hx-target=\"#page\"",
		"id=\"shell\"",
		"id=\"page\"",
		"<html",
	} {
		if !strings.Contains(shell, want) {
			t.Fatalf("boot missing %q\n%s", want, shell)
		}
	}
	for _, bad := range []string{"id=\"boot\"", "hx-swap-oob", "data-layout=", "data-landing=", "SLOPIFY_PAGE"} {
		if strings.Contains(shell, bad) {
			t.Fatalf("boot kept %q\n%s", bad, shell)
		}
	}
	if gen.count() < 1 || !gen.homeAt(0) {
		t.Fatalf("boot calls = %d home = %v", gen.count(), gen.homeAt(0))
	}

	again := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Cookie", cookie)
	h.ServeHTTP(again, req)
	if !strings.Contains(again.Body.String(), "Northwind Ferry") || strings.Contains(again.Body.String(), "id=\"boot\"") {
		t.Fatal("cached home should render the shell directly")
	}

	waitCached := func(path, marker string) {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Cookie", cookie)
			h.ServeHTTP(rec, req)
			body := rec.Body.String()
			if rec.Code == http.StatusOK && strings.Contains(body, marker) && !strings.Contains(body, "id=\"boot\"") {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("prefetch %s calls=%d body=%s", path, gen.count(), body)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	waitCached("/sailings", "Interpreted /sailings")
	waitCached("/pier/2", "Interpreted /pier/2")
	if gen.count() != 3 || gen.homeAt(1) {
		t.Fatalf("prefetch calls = %d home = %v", gen.count(), gen.homeAt(1))
	}

	cold := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/unlinked", nil)
	req.Header.Set("Cookie", cookie)
	h.ServeHTTP(cold, req)
	if cold.Code != http.StatusOK || !strings.Contains(cold.Body.String(), "hx-get=\"/unlinked\"") || !strings.Contains(cold.Body.String(), "hx-trigger=\"load\"") {
		t.Fatalf("cold route = %s", cold.Body.String())
	}
	if gen.count() != 3 {
		t.Fatalf("cold loader calls = %d", gen.count())
	}

	swapped := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/sailings", nil)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("HX-Request", "true")
	h.ServeHTTP(swapped, req)
	if swapped.Code != http.StatusOK {
		t.Fatalf("route boot status = %d", swapped.Code)
	}
	if !strings.Contains(swapped.Body.String(), "Interpreted /sailings") || !strings.Contains(swapped.Body.String(), "id=\"shell\"") {
		t.Fatalf("route boot = %s", swapped.Body.String())
	}
	if gen.count() != 3 {
		t.Fatalf("cached route calls = %d", gen.count())
	}

	part := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/sailings", nil)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Boosted", "true")
	h.ServeHTTP(part, req)
	frag := part.Body.String()
	if part.Code != http.StatusOK {
		t.Fatalf("fragment status = %d body = %s", part.Code, frag)
	}
	if strings.Contains(frag, "<html") {
		t.Fatal("fragment included a full document")
	}
	for _, want := range []string{
		"id=\"page\"",
		`class="deck"`,
		"Interpreted /sailings",
		"href=\"/pier/2\"",
		"<title>Sailings · Northwind Ferry</title>",
	} {
		if !strings.Contains(frag, want) {
			t.Fatalf("fragment missing %q\n%s", want, frag)
		}
	}
	if strings.Contains(frag, `class="mast"`) || strings.Contains(frag, "<style") {
		t.Fatalf("fragment included the frame\n%s", frag)
	}
	if gen.count() != 3 {
		t.Fatalf("route calls = %d", gen.count())
	}

	full := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/sailings", nil)
	req.Header.Set("Cookie", cookie)
	h.ServeHTTP(full, req)
	if gen.count() != 3 {
		t.Fatalf("cached route calls = %d", gen.count())
	}
	if !strings.Contains(full.Body.String(), "Northwind Ferry") || !strings.Contains(full.Body.String(), "<html") {
		t.Fatal("full route lost the session layout")
	}

	other := httptest.NewRecorder()
	h.ServeHTTP(other, httptest.NewRequest(http.MethodGet, "/", nil))
	if gen.count() != 3 || !strings.Contains(other.Body.String(), "id=\"boot\"") {
		t.Fatalf("new session loader calls = %d", gen.count())
	}

	fresh := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Cookie", cookieHeader(other))
	req.Header.Set("HX-Request", "true")
	h.ServeHTTP(fresh, req)
	if fresh.Code != http.StatusOK || gen.count() < 4 || !gen.homeAt(3) {
		t.Fatalf("new session boot status = %d calls = %d", fresh.Code, gen.count())
	}
}

type errGen struct{}

func (errGen) Complete(context.Context, bool, string, string) (string, error) {
	return "", context.Canceled
}

func TestBootFailureReplacesLoader(t *testing.T) {
	h := newServer(errGen{}).Handler()

	loader := httptest.NewRecorder()
	h.ServeHTTP(loader, httptest.NewRequest(http.MethodGet, "/", nil))
	if loader.Code != http.StatusOK || !strings.Contains(loader.Body.String(), "id=\"boot\"") {
		t.Fatalf("loader = %d", loader.Code)
	}

	boot := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Cookie", cookieHeader(loader))
	req.Header.Set("HX-Request", "true")
	h.ServeHTTP(boot, req)
	if boot.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", boot.Code, boot.Body.String())
	}
	if !strings.Contains(boot.Body.String(), "The site didn't come back") {
		t.Fatalf("body = %s", boot.Body.String())
	}
}

func TestNewSiteAvoidsPreviousFrame(t *testing.T) {
	gen := &fakeGen{}
	h := newServer(gen).Handler()

	loader := httptest.NewRecorder()
	h.ServeHTTP(loader, httptest.NewRequest(http.MethodGet, "/", nil))
	boot := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Cookie", cookieHeader(loader))
	req.Header.Set("HX-Request", "true")
	h.ServeHTTP(boot, req)
	if !strings.Contains(boot.Body.String(), `class="mast"`) {
		t.Fatal("first frame was not the generated markup")
	}

	reset := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/new-site", nil)
	req.Header.Set("Cookie", cookieHeader(loader))
	h.ServeHTTP(reset, req)
	avoid := cookieNamed(reset, "slopify_avoid")
	if reset.Code != http.StatusSeeOther || avoid == nil || strings.Count(avoid.Value, ".") != 4 {
		t.Fatalf("status=%d avoid=%#v", reset.Code, avoid)
	}

	next := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(avoid)
	h.ServeHTTP(next, req)
	if !strings.Contains(next.Body.String(), `id="boot"`) {
		t.Fatal("expected a fresh loader")
	}

	again := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Cookie", cookieHeader(next))
	req.Header.Set("HX-Request", "true")
	h.ServeHTTP(again, req)
	if again.Code != http.StatusOK {
		t.Fatalf("status=%d calls=%d", again.Code, gen.count())
	}
	found := false
	for i := 0; i < gen.count(); i++ {
		prompt := gen.callAt(i)
		if !strings.Contains(prompt, "PREVIOUS BRIEF") {
			continue
		}
		found = true
		if !gen.homeAt(i) {
			t.Fatal("avoidance used the route model")
		}
		if visualBrief(gen.callAt(0)) == visualBrief(prompt) {
			t.Fatalf("visual brief repeated\n%s", prompt)
		}
	}
	if !found {
		t.Fatal("next site was not given a new visual brief")
	}
}

func briefTheme(input string) string {
	i := strings.Index(input, "VISUAL BRIEF")
	if i < 0 {
		return ""
	}
	rest := input[i:]
	end := strings.Index(rest, "END BRIEF")
	if end < 0 {
		return ""
	}
	block := rest[:end]
	const mark = "color: "
	j := strings.Index(block, mark)
	if j < 0 {
		return ""
	}
	line := block[j+len(mark):]
	if n := strings.IndexAny(line, "\r\n"); n >= 0 {
		line = line[:n]
	}
	return strings.TrimSpace(line)
}

func visualBrief(prompt string) string {
	i := strings.Index(prompt, "VISUAL BRIEF")
	j := strings.Index(prompt, "END BRIEF")
	if i < 0 || j < i {
		return ""
	}
	return prompt[i:j]
}

func TestStateCRUDReturnsTheCallingFragment(t *testing.T) {
	gen := &fakeGen{}
	h := newServer(gen).Handler()

	post := httptest.NewRequest(http.MethodPost, "/state/items", strings.NewReader("name=milk"))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post.Header.Set("HX-Target", "list")
	post.Header.Set("HX-Trigger", "add")
	post.Header.Set("HX-Trigger-Name", "save")
	post.Header.Set("HX-Current-URL", "https://slopify.engehost.net/")
	post.Header.Set("HX-Prompt", "add milk")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, post)
	if res.Code != http.StatusOK {
		t.Fatalf("create status = %d body %s", res.Code, res.Body)
	}
	if res.Header().Get("Content-Type") != "text/html; charset=utf-8" || res.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers = %#v", res.Header())
	}
	if !strings.Contains(res.Body.String(), `id="list"`) {
		t.Fatalf("body = %s", res.Body)
	}
	if gen.homeAt(0) {
		t.Fatal("state used the home model")
	}
	created := gen.callAt(0)
	for _, want := range []string{
		"operation: create",
		"resource: items",
		"name=milk",
		"target: list",
		"trigger: add",
		"trigger name: save",
		"current url: https://slopify.engehost.net/",
		"prompt: add milk",
	} {
		if !strings.Contains(created, want) {
			t.Fatalf("prompt missing %q\n%s", want, created)
		}
	}
	id := stateRecordID(t, created, "name=milk")
	cookie := cookieHeader(res)
	if cookie == "" {
		t.Fatal("missing session cookie")
	}

	other := httptest.NewRecorder()
	h.ServeHTTP(other, httptest.NewRequest(http.MethodGet, "/state/items", nil))
	if strings.Contains(gen.callAt(gen.count()-1), "name=milk") {
		t.Fatal("a new session saw another session's record")
	}

	get := httptest.NewRequest(http.MethodGet, "/state/items", nil)
	get.Header.Set("Cookie", cookie)
	get.Header.Set("HX-Target", "#list")
	listed := httptest.NewRecorder()
	h.ServeHTTP(listed, get)
	if listed.Code != http.StatusOK || !strings.Contains(gen.callAt(gen.count()-1), "operation: list") || !strings.Contains(gen.callAt(gen.count()-1), "name=milk") {
		t.Fatalf("list status %d prompt %s", listed.Code, gen.callAt(gen.count()-1))
	}

	read := httptest.NewRequest(http.MethodGet, "/state/items/"+id, nil)
	read.Header.Set("Cookie", cookie)
	read.Header.Set("HX-Target", "list")
	got := httptest.NewRecorder()
	h.ServeHTTP(got, read)
	if got.Code != http.StatusOK || !strings.Contains(gen.callAt(gen.count()-1), "operation: read") || !strings.Contains(gen.callAt(gen.count()-1), "missing: no") {
		t.Fatalf("read status %d prompt %s", got.Code, gen.callAt(gen.count()-1))
	}

	patch := httptest.NewRequest(http.MethodPatch, "/state/items/"+id, strings.NewReader("qty=2"))
	patch.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	patch.Header.Set("Cookie", cookie)
	patch.Header.Set("HX-Target", "list")
	patched := httptest.NewRecorder()
	h.ServeHTTP(patched, patch)
	if patched.Code != http.StatusOK {
		t.Fatalf("patch status = %d", patched.Code)
	}
	put := httptest.NewRequest(http.MethodPut, "/state/items/"+id, strings.NewReader("note=cold"))
	put.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	put.Header.Set("Cookie", cookie)
	put.Header.Set("HX-Target", "list")
	updated := httptest.NewRecorder()
	h.ServeHTTP(updated, put)
	merged := gen.callAt(gen.count() - 1)
	if updated.Code != http.StatusOK || !strings.Contains(merged, "operation: update") || !strings.Contains(merged, "name=milk") || !strings.Contains(merged, "qty=2") || !strings.Contains(merged, "note=cold") {
		t.Fatalf("update status %d prompt %s", updated.Code, merged)
	}

	del := httptest.NewRequest(http.MethodDelete, "/state/items/"+id, nil)
	del.Header.Set("Cookie", cookie)
	del.Header.Set("HX-Target", "list")
	deleted := httptest.NewRecorder()
	h.ServeHTTP(deleted, del)
	if deleted.Code != http.StatusOK || !strings.Contains(gen.callAt(gen.count()-1), "operation: delete") {
		t.Fatalf("delete status %d prompt %s", deleted.Code, gen.callAt(gen.count()-1))
	}
	again := httptest.NewRequest(http.MethodGet, "/state/items", nil)
	again.Header.Set("Cookie", cookie)
	again.Header.Set("HX-Target", "list")
	empty := httptest.NewRecorder()
	h.ServeHTTP(empty, again)
	if !strings.Contains(gen.callAt(gen.count()-1), "records:\nnone") {
		t.Fatalf("after delete = %s", gen.callAt(gen.count()-1))
	}

	before := gen.count()
	missing := httptest.NewRecorder()
	h.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/state/Items", nil))
	if missing.Code != http.StatusNotFound || gen.count() != before {
		t.Fatalf("bad resource status %d calls %d", missing.Code, gen.count())
	}
	extraReq := httptest.NewRequest(http.MethodPost, "/state/items/abc", strings.NewReader("name=nope"))
	extraReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	extra := httptest.NewRecorder()
	h.ServeHTTP(extra, extraReq)
	if extra.Code != http.StatusOK || !strings.Contains(gen.callAt(gen.count()-1), "action: abc") {
		t.Fatalf("action status %d prompt %s", extra.Code, gen.callAt(gen.count()-1))
	}
}

func TestStateActionPath(t *testing.T) {
	gen := &fakeGen{}
	h := newServer(gen).Handler()

	draw := httptest.NewRequest(http.MethodGet, "/state/rounds/draw/vowel", nil)
	draw.Header.Set("HX-Target", "page")
	draw.Header.Set("HX-Current-URL", "https://slopify.engehost.net/play")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, draw)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `id="page"`) {
		t.Fatalf("draw status %d body %s", res.Code, res.Body)
	}
	got := gen.callAt(0)
	for _, want := range []string{"operation: act", "resource: rounds", "action: draw/vowel", "target: page"} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt missing %q\n%s", want, got)
		}
	}
	cookie := cookieHeader(res)

	next := httptest.NewRequest(http.MethodGet, "/state/rounds/draw/consonant", nil)
	next.Header.Set("Cookie", cookie)
	next.Header.Set("HX-Target", "#page")
	again := httptest.NewRecorder()
	h.ServeHTTP(again, next)
	second := gen.callAt(1)
	if again.Code != http.StatusOK || !strings.Contains(second, "action: draw/consonant") || !strings.Contains(second, "action=draw/vowel") || strings.Contains(second, "Interpreted page") {
		t.Fatalf("next status %d prompt %s", again.Code, second)
	}

	post := httptest.NewRequest(http.MethodPost, "/state/rounds/words", strings.NewReader("word=quiz"))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post.Header.Set("Cookie", cookie)
	post.Header.Set("HX-Target", "page")
	posted := httptest.NewRecorder()
	h.ServeHTTP(posted, post)
	third := gen.callAt(gen.count() - 1)
	if posted.Code != http.StatusOK || !strings.Contains(third, "operation: create") || !strings.Contains(third, "action: words") || !strings.Contains(third, "word=quiz") {
		t.Fatalf("post status %d prompt %s", posted.Code, third)
	}
}

func stateRecordID(t *testing.T, prompt, field string) string {
	t.Helper()
	i := strings.Index(prompt, "records:\n")
	if i < 0 {
		t.Fatalf("no records in %s", prompt)
	}
	for _, line := range strings.Split(prompt[i+len("records:\n"):], "\n") {
		if strings.TrimSpace(line) == "" {
			break
		}
		if strings.Contains(line, field) {
			return strings.Fields(line)[0]
		}
	}
	t.Fatalf("record %q not in %s", field, prompt)
	return ""
}

func TestNewSiteClearsSession(t *testing.T) {
	gen := &fakeGen{}
	h := newServer(gen).Handler()

	res := httptest.NewRecorder()
	h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/new-site", nil))
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", res.Code)
	}
	if loc := res.Header().Get("Location"); loc != "/" {
		t.Fatalf("location = %q", loc)
	}
	cleared := cookieNamed(res, site.CookieName)
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Fatalf("cookie = %#v", res.Result().Cookies())
	}
	if gen.count() != 0 {
		t.Fatalf("calls = %d", gen.count())
	}
}
