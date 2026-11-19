package template

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/DillonEnge/slopify/internal/site"
)

func TestDocumentRendersGeneratedFrame(t *testing.T) {
	s := site.Site{
		Name: "Northwind Ferry",
		Shell: `<style>header.mast{font-size:9rem}</style>` +
			`<header class="mast"><h1>Northwind Ferry</h1></header>` +
			`<!--SLOPIFY_PAGE-->` +
			`<footer><a href="/new-site" hx-boost="false">New site</a></footer>`,
		PageClass: "deck",
		HomeHTML:  `<main id="page" class="deck"><h1>Today's crossings</h1></main>`,
	}
	page := site.Page{Path: "/", Title: "Today's crossings", HTML: s.HomeHTML}
	var buf bytes.Buffer
	if err := Document(s, page).Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	body := buf.String()
	for _, want := range []string{
		`header.mast{font-size:9rem}`,
		`class="mast"`,
		`Today's crossings`,
		`id="page"`,
		`id="shell"`,
		`hx-history="false"`,
		`hx-target="#page"`,
		`hx-boost="false"`,
		`<html`,
		`/static/output.css`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("document missing %q\n%s", want, body)
		}
	}
	for _, bad := range []string{"data-landing=", "data-layout=", "SLOPIFY_PAGE"} {
		if strings.Contains(body, bad) {
			t.Fatalf("document kept %q\n%s", bad, body)
		}
	}

	inner := site.Page{
		Path:  "/sailings",
		Title: "Sailings",
		HTML:  `<main id="page" class="deck"><h1>Sailings</h1></main>`,
	}
	buf.Reset()
	if err := Fragment(s, inner).Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	frag := buf.String()
	if strings.Contains(frag, "<html") || strings.Contains(frag, `class="mast"`) || !strings.Contains(frag, `id="page"`) {
		t.Fatalf("fragment = %s", frag)
	}
	if !strings.Contains(frag, "Sailings · Northwind Ferry") && !strings.Contains(frag, "Sailings &#183; Northwind Ferry") {
		t.Fatalf("fragment title = %s", frag)
	}
}
