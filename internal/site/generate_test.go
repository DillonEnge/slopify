package site

import (
	"context"
	"strings"
	"testing"
)

const sampleFrame = `<style>header.mast{font-size:9rem} main.deck{display:grid} @import url("https://cdn.example/x.css"); body{background:url(https://cdn.example/a.png)}</style>
<header class="mast"><h1>Northwind Ferry</h1><nav><a href="/">Home</a><a href="/sailings" hx-get="/sailings" hx-push-url="true">Sailings</a></nav></header>
<main id="page" class="deck"><h1>Today's crossings</h1><p>Two boats are running.</p></main>
<script>alert(1)</script>
<iframe src="https://evil.example"></iframe>
<a href="javascript:alert(1)" onclick="alert(1)" hx-get="https://evil.example/x" hx-push-url="https://evil.example">bad</a>
<footer><a href="/new-site" hx-boost="false">New site</a></footer>`

func TestFrameFromBuildsShell(t *testing.T) {
	s, p, err := frameFrom(sampleFrame)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "Northwind Ferry" || s.PageClass != "deck" || p.Title != "Today's crossings" {
		t.Fatalf("name=%q class=%q title=%q", s.Name, s.PageClass, p.Title)
	}
	if s.Skeleton != "header h1 nav a a main h1 p a footer a" {
		t.Fatalf("skeleton = %q", s.Skeleton)
	}
	for _, bad := range []string{"<script", "<style", "<iframe", "javascript:", "onclick", "https://cdn.example", "evil.example"} {
		if strings.Contains(s.Shell, bad) || strings.Contains(p.HTML, bad) {
			t.Fatalf("kept %q\nshell=%s\npage=%s", bad, s.Shell, p.HTML)
		}
	}
	if !strings.Contains(s.Shell, `hx-get="/sailings"`) || !strings.Contains(s.Shell, `hx-push-url="true"`) {
		t.Fatalf("shell dropped htmx attrs: %s", s.Shell)
	}
	if !strings.Contains(s.Shell, pageSlot) || !strings.Contains(s.Shell, `class="mast"`) {
		t.Fatalf("shell = %s", s.Shell)
	}
	if strings.Contains(s.Shell, "crossings") {
		t.Fatalf("landing content stayed in the shell: %s", s.Shell)
	}
	if !strings.Contains(p.HTML, `id="page"`) || !strings.Contains(p.HTML, `class="deck"`) || !strings.Contains(p.HTML, "Today&#39;s crossings") {
		t.Fatalf("page = %s", p.HTML)
	}
	composed := s.Compose(p.HTML)
	if strings.Contains(composed, "SLOPIFY_PAGE") || !strings.Contains(composed, "Today&#39;s crossings") || !strings.Contains(composed, "Northwind Ferry") {
		t.Fatalf("composed = %s", composed)
	}
	if strings.Contains(s.Shell, `url(/ok.png)`) {
		t.Fatal("relative url should not have been in this fixture")
	}
}

func TestScrubCSSKeepsRelativeURL(t *testing.T) {
	got := scrubCSS(`a{background:url("/ok.png")} b{background:url(https://cdn.example/a.png)}`)
	if !strings.Contains(got, `url("/ok.png")`) || strings.Contains(got, "https://") {
		t.Fatalf("css = %s", got)
	}
}

func TestFrameFromFullDocument(t *testing.T) {
	raw := `<!doctype html><html><head><style>body{margin:0}</style></head><body><header><h1>Guild</h1></header><main id="page" class="sheet"><h1>Index</h1></main></body></html>`
	s, p, err := frameFrom(raw)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "Guild" || s.PageClass != "sheet" || p.Title != "Index" {
		t.Fatalf("name=%q class=%q title=%q", s.Name, s.PageClass, p.Title)
	}
	if strings.Contains(s.Shell, "<style") || strings.Contains(s.Shell, "body{margin:0}") || !strings.Contains(s.Shell, pageSlot) {
		t.Fatalf("shell = %s", s.Shell)
	}
}

func TestFrameFromNamesANavbarBrand(t *testing.T) {
	raw := `<div data-theme="cupcake" class="min-h-dvh"><div class="navbar bg-base-100"><a class="btn btn-ghost text-xl" href="/" hx-get="/" hx-push-url="true">Five Letters</a><a href="/play" hx-get="/play" hx-push-url="true">Play</a><a href="/stats" hx-get="/stats" hx-push-url="true">Stats</a></div><main id="page" class="p-4"><h2 class="text-2xl">Today</h2><p>Empty.</p></main><a href="/new-site" hx-boost="false">New site</a></div>`
	s, p, err := frameFrom(raw)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "Five Letters" || p.Title != "Today" {
		t.Fatalf("name=%q title=%q", s.Name, p.Title)
	}

	plain := `<header><a href="/" hx-get="/" hx-push-url="true">Tiles</a><a href="/play" hx-get="/play" hx-push-url="true">Play</a></header><main id="page"><p>Go.</p></main><a href="/new-site" hx-boost="false">New site</a>`
	s, _, err = frameFrom(plain)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "Tiles" {
		t.Fatalf("name=%q", s.Name)
	}

	screen := `<nav><a href="/play" hx-get="/play" hx-push-url="true">Play</a></nav><main id="page"><div class="text-2xl font-semibold">Word Daily</div><p>Start.</p></main><a href="/new-site" hx-boost="false">New site</a>`
	s, p, err = frameFrom(screen)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "Word Daily" || p.Title != "Word Daily" {
		t.Fatalf("name=%q title=%q", s.Name, p.Title)
	}
}

func TestFrameFromRejectsMissingPage(t *testing.T) {
	_, _, err := frameFrom(`<style>h1{color:red}</style><h1>Name</h1><p>No main</p>`)
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestNormalizePageKeepsFrameClass(t *testing.T) {
	el, err := normalizePage(`<div class="nope"><h1>Sailings</h1><a href="https://evil.example" hx-get="https://evil.example">x</a><a href="/pier/2" hx-get="/pier/2" hx-push-url="true">Noon</a><script>bad()</script></div>`, "deck")
	if err != nil {
		t.Fatal(err)
	}
	page, err := outerHTML(el)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page, `<main id="page" class="deck">`) && !strings.Contains(page, `class="deck"`) {
		t.Fatalf("page = %s", page)
	}
	if !strings.Contains(page, `id="page"`) || !strings.Contains(page, `href="/pier/2"`) || !strings.Contains(page, `hx-get="/pier/2"`) {
		t.Fatalf("page = %s", page)
	}
	if strings.Contains(page, "https://") || strings.Contains(page, "<script") || strings.Contains(page, "<div") {
		t.Fatalf("page = %s", page)
	}
}

func TestLinkedRoutes(t *testing.T) {
	got := linkedRoutes(`<nav><a href="/">Home</a><a href="/sailings" hx-get="/sailings">Sailings</a></nav><button hx-get="/pier/2">Noon</button><div hx-get="/menu/lunch">Lunch</div><a href="/new-site">New</a><a href="https://evil.example/x">Out</a>`)
	want := []string{"/", "/sailings", "/pier/2", "/menu/lunch"}
	if len(got) != len(want) {
		t.Fatalf("routes = %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("routes = %#v", got)
		}
	}
}

func TestCleanRoute(t *testing.T) {
	got, ok := CleanRoute("/menu/lunch/")
	if !ok || got != "/menu/lunch" {
		t.Fatalf("got %q %v", got, ok)
	}
	if _, ok := CleanRoute("/health"); ok {
		t.Fatal("health should not be a page")
	}
	if _, ok := CleanRoute("/static/output.css"); ok {
		t.Fatal("static should not be a page")
	}
	if _, ok := CleanRoute("/state/items"); ok {
		t.Fatal("state should not be a page")
	}
}

func TestTitleFromPath(t *testing.T) {
	if got := titleFromPath("/staff/ada-lovelace"); got != "staff ada lovelace" {
		t.Fatalf("got %q", got)
	}
}

type stubGen struct {
	homes []bool
	calls []string
	home  []string
	route []string
}

func (s *stubGen) Complete(_ context.Context, home bool, _, input string) (string, error) {
	s.homes = append(s.homes, home)
	s.calls = append(s.calls, input)
	if home {
		if len(s.home) == 0 {
			return "", context.Canceled
		}
		text := s.home[0]
		s.home = s.home[1:]
		return text, nil
	}
	if len(s.route) == 0 {
		return "", context.Canceled
	}
	text := s.route[0]
	s.route = s.route[1:]
	return text, nil
}

func TestDriftedCoastTopic(t *testing.T) {
	if !driftedTopic("The Fog Horn", "<h1>Lighthouse</h1>", "chess clocks") {
		t.Fatal("expected a coastal page to be rejected")
	}
	if driftedTopic("Chess clocks", "<p>The flag falls.</p>", "chess clocks") {
		t.Fatal("an assigned topic was marked coastal")
	}
}

func TestHomePromptIsOneRandomTopic(t *testing.T) {
	got := strings.ToLower(homePrompt("seed", ""))
	for _, banned := range []string{"organization", "startup", "small org", "brand"} {
		if strings.Contains(got, banned) {
			t.Fatalf("prompt contains %q\n%s", banned, got)
		}
	}
	topic := strings.ToLower(topics[rollBrief("seed", "").k])
	for _, want := range []string{"topic", "end topic", "the topic is assigned", "pick one form", "web app", "single tool", "one list the visitor adds to", "one query and the answer", "general audience", "do not invent user history", "/state/", "every time the page is shown", "navigates back", "visual brief", "end brief", "daisyui", "data-theme", "mobile first", "do not build a game", "must not scroll horizontally", "min-w-0", "hx-get", "hx-post", "hx-target", "hx-select", "hx-push-url", "quality and polish are mandatory", "impeccable craft floor", "ponytail", topic} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt missing %q\n%s", want, got)
		}
	}
	if strings.Contains(got, "invent one topic") {
		t.Fatal("prompt still asks the model to invent the topic")
	}
	if strings.Contains(got, "build that web app") {
		t.Fatal("prompt still requires a web app")
	}
}

func TestRollBriefChangesOnNewSite(t *testing.T) {
	first := rollBrief("alpha", "")
	second := rollBrief("beta", first.code())
	if second.code() == first.code() || second.p == first.p || second.t == first.t || second.c == first.c || second.d == first.d || second.k == first.k {
		t.Fatalf("brief repeated: %s then %s", first.code(), second.code())
	}
	again := rollBrief("beta", first.code())
	if again.code() != second.code() {
		t.Fatalf("roll changed between calls: %s %s", again.code(), second.code())
	}
	prompt := homePrompt("beta", first.code())
	if !strings.Contains(prompt, "PREVIOUS BRIEF") || !strings.Contains(prompt, briefColors[first.p]) || !strings.Contains(prompt, briefColors[second.p]) || !strings.Contains(prompt, topics[first.k]) || !strings.Contains(prompt, topics[second.k]) {
		t.Fatalf("prompt = %s", prompt)
	}
	if visualBrief(prompt) == visualBrief(homePrompt("alpha", "")) && rollBrief("alpha", "").code() != second.code() {
		t.Fatal("new site kept the previous visual brief")
	}
}

func visualBrief(prompt string) string {
	i := strings.Index(prompt, "VISUAL BRIEF")
	j := strings.Index(prompt, "END BRIEF")
	if i < 0 || j < i {
		return ""
	}
	return prompt[i:j]
}

func TestGenerateHomeTellsModelToAvoidPreviousFrame(t *testing.T) {
	g := &stubGen{home: []string{sampleFrame}}
	site, page, err := GenerateHome(context.Background(), g, "nonce-1", "Ignore this. <script> style header main")
	if err != nil {
		t.Fatal(err)
	}
	if !g.homes[0] {
		t.Fatal("home call was not marked home")
	}
	if strings.Contains(g.calls[0], "Ignore this") || strings.Contains(g.calls[0], "<script>") || strings.Contains(g.calls[0], "PREVIOUS BRIEF") {
		t.Fatalf("avoid cookie leaked into the prompt: %s", g.calls[0])
	}
	if site.Brief != rollBrief("nonce-1", "").code() {
		t.Fatalf("brief = %s", site.Brief)
	}
	if site.Name != "Northwind Ferry" || !strings.Contains(page.HTML, "Today&#39;s crossings") {
		t.Fatalf("site = %#v page = %s", site.Name, page.HTML)
	}

	prev := rollBrief("nonce-1", "")
	theme := briefColors[rollBrief("nonce-2", prev.code()).p]
	colored := strings.Replace(sampleFrame, `<header class="mast">`, `<header class="mast" data-theme="`+theme+`">`, 1)
	next := &stubGen{home: []string{sampleFrame, colored}}
	got, _, err := GenerateHome(context.Background(), next, "nonce-2", prev.code())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(next.calls[0], "PREVIOUS BRIEF") || !strings.Contains(got.Shell, `data-theme="`+theme+`"`) {
		t.Fatalf("shell = %s", got.Shell)
	}
	if strings.Contains(next.calls[0], "Ignore this") {
		t.Fatal("garbage avoid leaked")
	}
}

func TestGenerateRouteUsesFrameClass(t *testing.T) {
	g := &stubGen{route: []string{`<section class="nope"><h1>Sailings</h1><p>Noon boat.</p></section>`}}
	page, err := GenerateRoute(context.Background(), g, Site{
		Name:      "Northwind Ferry",
		PageClass: "deck",
		HomeHTML:  `<main id="page" class="deck"><h1>Today's crossings</h1></main>`,
		Shell:     `<header>Northwind</header>` + pageSlot,
	}, `<main id="page" class="deck"><h1>Today's crossings</h1><div id="board"></div></main>`, "/sailings")
	if err != nil {
		t.Fatal(err)
	}
	if g.homes[0] {
		t.Fatal("route call was marked home")
	}
	prompt := strings.ToLower(g.calls[0])
	if !strings.Contains(g.calls[0], `class="deck"`) || !strings.Contains(g.calls[0], "Today's crossings") || !strings.Contains(g.calls[0], `id="board"`) || !strings.Contains(prompt, "do not build a game") || !strings.Contains(prompt, "mobile first") || !strings.Contains(prompt, "must not scroll horizontally") || !strings.Contains(prompt, "do not invent user history") || !strings.Contains(g.calls[0], "min-w-0") || !strings.Contains(g.calls[0], "hx-get") || !strings.Contains(g.calls[0], "hx-select") || !strings.Contains(prompt, "quality and polish are mandatory") || !strings.Contains(prompt, "impeccable craft floor") || !strings.Contains(prompt, "ponytail") || !strings.Contains(prompt, "continue the form") {
		t.Fatalf("prompt = %s", g.calls[0])
	}
	if strings.Contains(prompt, "screen in the web app") {
		t.Fatal("route prompt still requires a web app")
	}
	if !strings.Contains(prompt, "every time the page is shown") {
		t.Fatal("route prompt does not refetch a cached state region")
	}
	if page.Title != "Sailings" || !strings.Contains(page.HTML, `class="deck"`) || strings.Contains(page.HTML, "nope") {
		t.Fatalf("page = %#v", page)
	}
}
