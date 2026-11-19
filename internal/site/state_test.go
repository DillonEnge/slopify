package site

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

type promptGen struct {
	reply  string
	fail   int
	inputs []string
	homes  []bool
	instr  []string
}

func (g *promptGen) Complete(_ context.Context, home bool, instructions, input string) (string, error) {
	g.inputs = append(g.inputs, input)
	g.homes = append(g.homes, home)
	g.instr = append(g.instr, instructions)
	if g.fail > 0 {
		g.fail--
		return "not html", nil
	}
	if g.reply == "" {
		return `<section id="list">ok</section>`, nil
	}
	return g.reply, nil
}

func TestApplyKeepsRecordsAndUsesCallingHTMX(t *testing.T) {
	g := &promptGen{reply: `<script>alert(1)</script><div><section id="list"><p>kept</p><button onclick="bad()" hx-post="https://evil.example/x">x</button></section></div>`}
	s := &Session{
		lastHTML: `<main id="page"><section id="list"><p>stale</p></section></main>`,
		pages: map[string]Page{
			"/play": {Path: "/play", HTML: `<main id="page"><section id="list"><p>Empty</p></section></main>`},
		},
		site: &Site{Name: "Words", PageClass: "board"},
	}
	call := HTMXCall{
		Target:      "#list",
		Trigger:     "add",
		TriggerName: "save",
		CurrentURL:  "https://slopify.engehost.net/",
		Prompt:      "add milk",
		Referer:     "https://slopify.engehost.net/play",
	}
	frag, err := s.Apply(context.Background(), g, "create", "items", "", map[string]string{"name": "milk"}, call)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(frag, "script") || strings.Contains(frag, "onclick") || strings.Contains(frag, "evil.example") {
		t.Fatalf("fragment = %s", frag)
	}
	if !strings.Contains(frag, `id="list"`) || !strings.Contains(frag, "kept") || strings.Contains(frag, "<div>") {
		t.Fatalf("fragment = %s", frag)
	}
	if g.homes[0] {
		t.Fatal("state used the home model")
	}
	if !strings.Contains(g.instr[0], "one HTML element") {
		t.Fatalf("instructions = %s", g.instr[0])
	}
	for _, want := range []string{
		"operation: create",
		"resource: items",
		"site: Words",
		"page class: board",
		"target: list",
		"trigger: add",
		"trigger name: save",
		"current url: https://slopify.engehost.net/",
		"prompt: add milk",
		"name=milk",
		"<p>Empty</p>",
	} {
		if !strings.Contains(g.inputs[0], want) {
			t.Fatalf("prompt missing %q\n%s", want, g.inputs[0])
		}
	}
	if strings.Contains(g.inputs[0], "stale") {
		t.Fatal("prompt used the remembered page instead of the referer cache")
	}
	if len(s.records["items"]) != 1 {
		t.Fatalf("records = %#v", s.records["items"])
	}
	id := s.records["items"][0].ID

	if _, err := s.Apply(context.Background(), g, "update", "items", id, map[string]string{"qty": "2"}, HTMXCall{Target: "list"}); err != nil {
		t.Fatal(err)
	}
	got := s.records["items"][0].Fields
	if got["name"] != "milk" || got["qty"] != "2" {
		t.Fatalf("fields = %#v", got)
	}
	if !strings.Contains(g.inputs[1], "operation: update") || !strings.Contains(g.inputs[1], "missing: no") {
		t.Fatalf("update prompt = %s", g.inputs[1])
	}

	if _, err := s.Apply(context.Background(), g, "read", "items", "missingid", nil, HTMXCall{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(g.inputs[2], "operation: read") || !strings.Contains(g.inputs[2], "missing: yes") || !strings.Contains(g.inputs[2], "target: items") {
		t.Fatalf("read prompt = %s", g.inputs[2])
	}
	if len(s.records["items"]) != 1 {
		t.Fatalf("read stored a record: %#v", s.records["items"])
	}

	if _, err := s.Apply(context.Background(), g, "delete", "items", id, nil, HTMXCall{Target: "list"}); err != nil {
		t.Fatal(err)
	}
	if len(s.records["items"]) != 0 {
		t.Fatalf("records = %#v", s.records["items"])
	}
	if _, err := s.Apply(context.Background(), g, "list", "items", "", nil, HTMXCall{Target: "list"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(g.inputs[4], "operation: list") || !strings.Contains(g.inputs[4], "records:\nnone") {
		t.Fatalf("list prompt = %s", g.inputs[4])
	}
}

func TestApplyRetriesUntilTheFragmentParses(t *testing.T) {
	g := &promptGen{fail: 1, reply: `<div class="box">fresh</div>`}
	s := &Session{}
	frag, err := s.Apply(context.Background(), g, "list", "items", "", nil, HTMXCall{})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.inputs) != 2 {
		t.Fatalf("calls = %d", len(g.inputs))
	}
	if !strings.Contains(frag, `id="items"`) || !strings.Contains(frag, "fresh") {
		t.Fatalf("fragment = %s", frag)
	}
}

func TestFragmentDropsLoadTrigger(t *testing.T) {
	raw := `<section id="puzzle" hx-get="/state/puzzle" hx-trigger="load" hx-target="#puzzle" hx-select="#puzzle" hx-swap="outerHTML">Board</section>`
	frag, err := fragmentFrom(raw, "puzzle")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(frag, "hx-trigger") || strings.Contains(frag, "load") {
		t.Fatalf("fragment = %s", frag)
	}
	if !strings.Contains(frag, `hx-get="/state/puzzle"`) || !strings.Contains(frag, `hx-target="#puzzle"`) {
		t.Fatalf("fragment = %s", frag)
	}

	mixed, err := fragmentFrom(`<section id="puzzle" hx-get="/state/puzzle/clues/1a" hx-trigger="click, load once, every 2s" hx-target="#puzzle">1</section>`, "puzzle")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mixed, `hx-trigger="click"`) || strings.Contains(mixed, "load") || strings.Contains(mixed, "every") {
		t.Fatalf("fragment = %s", mixed)
	}
}

func TestParseStatePathAcceptsActions(t *testing.T) {
	op, resource, id, action, ok := ParseStatePath(http.MethodGet, "rounds/draw/vowel")
	if !ok || op != "act" || resource != "rounds" || id != "" || action != "draw/vowel" {
		t.Fatalf("got %s %s %s %s %v", op, resource, id, action, ok)
	}
	op, resource, id, action, ok = ParseStatePath(http.MethodPost, "rounds/words")
	if !ok || op != "create" || resource != "rounds" || action != "words" {
		t.Fatalf("post got %s %s %s %s %v", op, resource, id, action, ok)
	}
	if _, _, _, _, ok := ParseStatePath(http.MethodGet, "Rounds"); ok {
		t.Fatal("uppercase resource should not match")
	}
}

func TestApplyRejectsABadResource(t *testing.T) {
	s := &Session{}
	if _, err := s.Apply(context.Background(), &promptGen{}, "list", "Items", "", nil, HTMXCall{}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestCachedPageRefetchesItsStateRegion(t *testing.T) {
	raw := `<main id="page">` +
		`<section id="items" hx-get="/state/items" hx-target="#items" hx-select="#items"><p>empty</p></section>` +
		`<section id="one" hx-get="/state/items/abc" hx-target="this"><p>one</p></section>` +
		`<section id="kept" hx-get="/state/items" hx-trigger="click" hx-target="#kept">rows</section>` +
		`<section id="board" hx-get="/state/rounds/draw/vowel" hx-target="#board">letters</section>` +
		`<button type="button" id="draw" hx-get="/state/items" hx-target="#items" hx-select="#items">Reload</button>` +
		`<div id="open" role="button" hx-get="/state/items" hx-target="#open">Open</div>` +
		`<div id="other" hx-get="/state/items" hx-target="#items">no</div>` +
		`</main>`
	s := &Session{
		site:  &Site{Name: "List", HomeHTML: raw, Shell: "<!--SLOPIFY_PAGE-->"},
		pages: map[string]Page{"/": {Path: "/", HTML: raw}},
	}
	_, page, err := s.View(context.Background(), &promptGen{}, "/")
	if err != nil {
		t.Fatal(err)
	}
	if s.pages["/"].HTML != raw {
		t.Fatal("cached page was rewritten")
	}
	_, again, err := s.View(context.Background(), &promptGen{}, "/")
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{page.HTML, again.HTML} {
		nodes, err := parseInput(body)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		var walk func(*html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.ElementNode {
				if id := getAttr(n, "id"); id != "" {
					got[id] = getAttr(n, "hx-trigger")
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
		for _, n := range nodes {
			walk(n)
		}
		if got["items"] != "load" || got["one"] != "load" || got["kept"] != "load, click" {
			t.Fatalf("triggers = %#v\n%s", got, body)
		}
		for _, id := range []string{"board", "draw", "open", "other"} {
			if strings.Contains(got[id], "load") {
				t.Fatalf("%s armed: %#v", id, got)
			}
		}
	}
}
