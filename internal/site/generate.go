package site

import (
	"context"
	"fmt"
	"path"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const homeInstructions = "You design one mass-appeal site as one HTML fragment. The TOPIC is assigned. Pick one form from the prompt. Web app is one of those forms. Do not build a game. Follow the VISUAL BRIEF. The layout is mobile first. Use DaisyUI components and Tailwind utilities. Every anchor, button, and clickable div uses hx-get. Quality and polish are mandatory. Follow the Impeccable craft floor and the Ponytail style. Reply with the fragment and nothing else."
const routeInstructions = "You write one main element that continues the site already on the page. Keep its form. Do not build a game. You can see the current page. Keep the ids its controls swap. The screen is mobile first. Every anchor, button, and clickable div uses hx-get. Quality and polish are mandatory. Follow the Impeccable craft floor and the Ponytail style. Reply with the element and nothing else."

// craftRules is the Impeccable craft floor plus Ponytail markup, trimmed to what this pipeline can render.
const craftRules = `Follow the Impeccable craft floor and write the markup in the Ponytail style. The visual brief wins when it conflicts with a habit. Impeccable: one clear hierarchy, tight groups and generous separation, more space above a heading than under it, body copy in a readable measure, theme colors with real contrast, controls that name their action, and hover and focus from DaisyUI. No eyebrow label over a heading, no nested cards, no grid of identical icon-heading-text cards, no gradient text, no glass blur, no emoji or unicode as icons, and no decorative monospace unless the brief's type is mono or the text is data. Ponytail: the fewest elements that still look finished. Reuse a DaisyUI component or a Tailwind utility before adding a wrapper. No empty section, no scaffolding for later, and no class you invented. Skip anything the page does not need.`

const pageSlot = "<!--SLOPIFY_PAGE-->"

type Generator interface {
	// home is true only for the base route. Subroutes pass false.
	Complete(ctx context.Context, home bool, instructions, input string) (string, error)
}

// Site is one generated frame. Shell is that frame's HTML with pageSlot
// where the page element belongs. HomeHTML is the landing page element.
type Site struct {
	Name      string
	Shell     string
	PageClass string
	HomeHTML  string
	Skeleton  string
	Brief     string
}

type Page struct {
	Path  string
	Title string
	HTML  string
}

func (s Site) Compose(pageHTML string) string {
	if strings.Contains(s.Shell, pageSlot) {
		return strings.Replace(s.Shell, pageSlot, pageHTML, 1)
	}
	return s.Shell + pageHTML
}

func PlainPage(className, title, body string) string {
	class := ""
	if className != "" {
		class = ` class="` + html.EscapeString(className) + `"`
	}
	return `<main id="page"` + class + `><h1>` + html.EscapeString(title) + `</h1><p>` + html.EscapeString(body) + `</p></main>`
}

func GenerateHome(ctx context.Context, g Generator, nonce, avoidCode string) (Site, Page, error) {
	b := rollBrief(nonce, avoidCode)
	input := homePrompt(nonce, avoidCode)
	text, err := g.Complete(ctx, true, homeInstructions, input)
	if err != nil {
		return Site{}, Page{}, err
	}
	site, page, ferr := frameFrom(text)
	if ferr != nil || !usesBriefTheme(site.Shell, b) || driftedTopic(site.Name, site.HomeHTML, topics[b.k]) {
		text2, err2 := g.Complete(ctx, true, homeInstructions, input+"\n\nPut the brief theme on the root as data-theme. Put the site name in the navbar or in a heading outside main. Use DaisyUI components and Tailwind utilities. Quality and polish are mandatory. Do not write a style element or hex colors. Stay on the assigned topic and the form you picked. Do not write about fog, lighthouses, harbors, ships, or the sea. Reply with only the HTML fragment.")
		if err2 != nil {
			if ferr != nil {
				return Site{}, Page{}, err2
			}
		} else if site2, page2, err3 := frameFrom(text2); err3 == nil {
			site, page, ferr = site2, page2, nil
		} else if ferr != nil {
			return Site{}, Page{}, err3
		}
	}
	if ferr != nil {
		return Site{}, Page{}, ferr
	}
	site.Brief = b.code()
	return site, page, nil
}

func homePrompt(nonce, avoidCode string) string {
	b := rollBrief(nonce, avoidCode)
	avoid := ""
	if prev, ok := parseBrief(avoidCode); ok {
		avoid = fmt.Sprintf(`
PREVIOUS BRIEF, forbidden. Do not reuse this color, this composition, or this topic:
color: %s
composition: %s
topic: %s
END PREVIOUS
`, briefColors[prev.p], briefCompositions[prev.c], topics[prev.k])
	}
	return fmt.Sprintf(`TOPIC
%s
END TOPIC

VISUAL BRIEF
color: %s
type: %s
composition: %s
density: %s
END BRIEF
%s
The TOPIC is assigned. Do not replace it. It is for a general audience. Someone can use the first screen without a manual. Pick one form and build only that:
- web app: several screens, chrome, and a list, form, or table
- single tool: one screen that does the task and shows the result
- log: one list the visitor adds to, empty at the start
- lookup: one query and the answer
Do not mix forms. Do not build a game, a puzzle, a quiz, or a scoreboard. It is not an article, a magazine, a store, a company homepage, or a niche trade ledger. Do not write about fog, lighthouses, harbors, ships, or the sea.

The name and the copy belong to the topic. A web app uses paths such as /list, /new, and /how-to. The other forms stay on this screen. The visual brief controls the DaisyUI theme, the type utilities, the composition, and the density. Follow that composition as the layout of the form you picked. A table is the empty list, not a scoreboard.

Design the frame yourself as HTML that uses the Tailwind and DaisyUI classes already loaded on the page. Return one HTML fragment. The fragment is the chrome and the first screen together.

Quality and polish are mandatory. Ship a finished screen: consistent spacing, aligned regions, a clear type hierarchy, copy that is specific to the topic, and DaisyUI components that belong together. Do not invent user history. The visitor has done nothing yet. No prior entries, streaks, saved items, or "welcome back." A list is empty, a timer is at zero, and totals are zero. The chrome may feel like a new account, with nothing in it. A web app may include signup and login: labeled inputs with required, an email field uses type="email", and a password uses minlength. The submit button uses hx-get. A missing or invalid field swaps an error into the form. A valid submit opens this same fresh site. No placeholder text, no default browser controls, no marketing hero, no slogan, no pricing table, and no unfinished layout. An empty state the visitor has not filled is correct.

%s

The layout is mobile first. Write the phone layout with unprefixed utilities: one column, type that fits the screen, padding, and tap targets of at least 44px. On a phone the menu is above the workspace. Do not put the form or the list in a row beside a sidebar, and do not set h-screen overflow-hidden on the shell. The page must not scroll horizontally. Do not use w-screen or a fixed width wider than the screen. Put min-w-0 and max-w-full on flex and grid children. Long words wrap. A wide table scrolls inside one overflow-x-auto element that is itself max-w-full. From the md breakpoint, add md: and lg: utilities so wider screens reach the visual brief's composition.

A control that only opens another screen sets hx-get and hx-push-url="true" and does not set hx-target. Anchors also set href to that same path. Anything that creates, reads, updates, or deletes data uses /state/ and does not invent a page path for that data. A list is hx-get="/state/{resource}". A named action is a path under that resource, such as hx-post="/state/items" or hx-delete="/state/items/{id}". Update is hx-put="/state/{resource}/{id}". Delete is hx-delete="/state/{resource}/{id}". The region that shows the records has an id and sets hx-get="/state/{resource}" on itself, with hx-target and hx-select set to that same id. The page is cached. That region is fetched again every time the page is shown, including when the visitor navigates back. Do not set hx-trigger="load" or hx-trigger="every ..." yourself. A swapped element that still has hx-trigger="load" requests itself again. The first screen may draw the empty list, blank form, or zeroed clock until that fetch returns. Set hx-target and hx-select to the region id and hx-push-url="false". A form puts hx-post on the form, and its button is type="submit". A clickable div also sets role="button" and tabindex="0". A button is type="button" unless it submits a form. Do not set hx-on. Do not set hx-boost="false" except on the New site anchor. Do not leave a control that does nothing.

The fragment must include:
- a root element with data-theme set to the brief theme and the brief's font class
- DaisyUI components that fit the form you picked. A web app has a navbar or menu and links to its other screens. A single tool, a log, or a lookup is one screen: a form, a list, or a query, and no extra screens. Use tabs, fieldset, input, select, textarea, badge, dropdown, alert, steps, breadcrumbs, collapse, or join when the form needs them. Do not build the screen from a hero and a row of stats
- Tailwind utilities for layout: flex, grid, gap, padding, max width, and type scale
- no style element, no hex colors, and no invented class names
- one main element with id="page" that holds only the first screen
- when the form is a web app, links to its other screens outside that main element, each with href and hx-get
- an anchor outside main, with no hx-get: <a href="/new-site" hx-boost="false">New site</a>

The site name is a heading outside main, or the name in the navbar.
No html, head, body, script, iframe, link, or event-handler attributes.
No external URLs.
`, topics[b.k], briefColors[b.p], briefTypes[b.t], briefCompositions[b.c], briefDensity[b.d], avoid, craftRules)
}

func GenerateRoute(ctx context.Context, g Generator, s Site, sourceHTML, route string) (Page, error) {
	input := routePrompt(s, sourceHTML, route)
	text, err := g.Complete(ctx, false, routeInstructions, input)
	if err != nil {
		return Page{}, err
	}
	page, ferr := pageFrom(text, route, s.PageClass)
	if ferr != nil {
		text2, err2 := g.Complete(ctx, false, routeInstructions, input+"\n\nQuality and polish are mandatory. Reply with only the main element.")
		if err2 != nil {
			return Page{}, err2
		}
		page, ferr = pageFrom(text2, route, s.PageClass)
	}
	if ferr != nil {
		return Page{}, ferr
	}
	return page, nil
}

func routePrompt(s Site, sourceHTML, route string) string {
	classRule := `Give it id="page" and no class attribute.`
	if s.PageClass != "" {
		classRule = fmt.Sprintf(`Give it id="page" and class="%s".`, s.PageClass)
	}
	if strings.TrimSpace(sourceHTML) == "" {
		sourceHTML = s.HomeHTML
	}
	return fmt.Sprintf(`path: %s
site: %s

Current page:
%s

Return one main element for this path. %s

The current page is the whole screen the user is looking at, chrome and workspace. A control on it requested this path. Keep every id that control swaps. If it has hx-select="#list" for this path, this main must contain that same element, updated to show the result of the control. Do not drop ids used as swap targets, and do not invent a target that is not on the current page. Put the record id in the path so a different record is a different path. Do not invent user history. Show only what this click just did. Do not add saved items or a name that was not already on the current page.

%s

Quality and polish are mandatory: match the frame's spacing, type, and components, and write finished copy for this path. No placeholder text and no unstyled regions. Continue the form already on the current page. Do not switch it. Do not turn the page into an article or a shop. Do not build a game. Render this path so it is usable. Do not describe the path. The page is mobile first: unprefixed utilities stack for a phone, with tap targets of at least 44px. On a phone the menu is above the workspace. Do not put the form or the list in a row beside a sidebar. The page must not scroll horizontally. Do not use w-screen or a fixed width wider than the screen. Put min-w-0 and max-w-full on flex and grid children. Long words wrap. A wide table scrolls inside one overflow-x-auto element that is itself max-w-full. From md: and lg:, follow the frame's wider layout. A control that only opens another screen sets hx-get and hx-push-url="true" and does not set hx-target. Data changes use /state/{resource} or /state/{resource}/{action}, such as /state/rounds/draw/vowel: hx-get to list or run an action, hx-post to create or submit a form, hx-put="/state/{resource}/{id}" to update, and hx-delete to delete. The region that shows the records has an id and sets hx-get="/state/{resource}" on itself, with hx-target and hx-select set to that same id. The page is cached. That region is fetched again every time the page is shown, including when the visitor navigates back. Do not set hx-trigger="load" or hx-trigger="every ..." yourself. Set hx-target and hx-select to that region id and hx-push-url="false". A form puts hx-post on the form and its button is type="submit". Anchors also set href to the same path. A clickable div also has role="button" and tabindex="0". A button is type="button" unless it submits a form. Do not set hx-on or hx-boost. Use Tailwind and DaisyUI classes the frame already uses. Do not add a style element or hex colors. No style, script, html, head, or external URLs.`,
		route, s.Name, s.Compose(sourceHTML), classRule, craftRules)
}

func frameFrom(raw string) (Site, Page, error) {
	nodes, err := parseInput(raw)
	if err != nil {
		return Site{}, Page{}, err
	}
	nodes = scrubList(nodes)
	pageEl := findPage(nodes)
	if pageEl == nil {
		return Site{}, Page{}, fmt.Errorf("site frame: missing page")
	}
	name := siteName(nodes, pageEl)
	if name == "" {
		return Site{}, Page{}, fmt.Errorf("site frame: missing name")
	}
	setAttr(pageEl, "id", "page")
	className := getAttr(pageEl, "class")
	skel := skeleton(nodes)
	pageHTML, err := outerHTML(pageEl)
	if err != nil {
		return Site{}, Page{}, err
	}
	title := headingInside(pageEl)
	if title == "" {
		title = name
	}
	nodes = replaceNode(nodes, pageEl, &html.Node{Type: html.CommentNode, Data: "SLOPIFY_PAGE"})
	shell, err := renderNodes(nodes)
	if err != nil {
		return Site{}, Page{}, err
	}
	if !strings.Contains(shell, pageSlot) {
		return Site{}, Page{}, fmt.Errorf("site frame: missing page slot")
	}
	return Site{
			Name:      name,
			Shell:     shell,
			PageClass: className,
			HomeHTML:  pageHTML,
			Skeleton:  skel,
		}, Page{
			Path:  "/",
			Title: title,
			HTML:  pageHTML,
		}, nil
}

func pageFrom(raw, route, className string) (Page, error) {
	el, err := normalizePage(raw, className)
	if err != nil {
		return Page{}, err
	}
	pageHTML, err := outerHTML(el)
	if err != nil {
		return Page{}, err
	}
	title := headingInside(el)
	if title == "" {
		title = titleFromPath(route)
	}
	return Page{Path: route, Title: title, HTML: pageHTML}, nil
}

func normalizePage(raw, className string) (*html.Node, error) {
	nodes, err := parseInput(raw)
	if err != nil {
		return nil, err
	}
	nodes = scrubList(nodes)
	el := findPage(nodes)
	if el == nil {
		for _, n := range nodes {
			if n.Type == html.ElementNode {
				el = n
				break
			}
		}
	}
	if el == nil {
		return nil, fmt.Errorf("page html: missing element")
	}
	if el.Parent != nil {
		el.Parent.RemoveChild(el)
	}
	el.Data = "main"
	el.DataAtom = atom.Main
	setAttr(el, "id", "page")
	if className != "" {
		setAttr(el, "class", className)
	} else {
		delAttr(el, "class")
	}
	if strings.TrimSpace(textContent(el)) == "" {
		return nil, fmt.Errorf("page html: empty")
	}
	return el, nil
}

func parseInput(raw string) ([]*html.Node, error) {
	raw = stripFence(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty html")
	}
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "<html") || strings.Contains(lower, "<body") || strings.Contains(lower, "<head") {
		doc, err := html.Parse(strings.NewReader(raw))
		if err != nil {
			return nil, err
		}
		var nodes []*html.Node
		if head := findTag(doc, "head"); head != nil {
			nodes = append(nodes, detachChildren(head, func(n *html.Node) bool {
				return n.Type == html.ElementNode && n.Data == "style"
			})...)
		}
		if body := findTag(doc, "body"); body != nil {
			nodes = append(nodes, detachChildren(body, func(*html.Node) bool { return true })...)
		}
		return nodes, nil
	}
	ctx := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	return html.ParseFragment(strings.NewReader(raw), ctx)
}

func detachChildren(parent *html.Node, keep func(*html.Node) bool) []*html.Node {
	var taken []*html.Node
	for c := parent.FirstChild; c != nil; {
		next := c.NextSibling
		if keep(c) {
			parent.RemoveChild(c)
			taken = append(taken, c)
		}
		c = next
	}
	return taken
}

func findTag(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if f := findTag(c, tag); f != nil {
			return f
		}
	}
	return nil
}

var droppedTags = map[string]bool{
	"script":   true,
	"style":    true,
	"iframe":   true,
	"object":   true,
	"embed":    true,
	"link":     true,
	"meta":     true,
	"base":     true,
	"noscript": true,
}

func scrubList(nodes []*html.Node) []*html.Node {
	out := make([]*html.Node, 0, len(nodes))
	for _, n := range nodes {
		if dropNode(n) {
			continue
		}
		scrub(n)
		out = append(out, n)
	}
	return out
}

func dropNode(n *html.Node) bool {
	return n.Type == html.ElementNode && droppedTags[n.Data]
}

func scrub(n *html.Node) {
	if n.Type == html.ElementNode {
		scrubAttrs(n)
		if n.Data == "style" {
			for t := n.FirstChild; t != nil; t = t.NextSibling {
				if t.Type == html.TextNode {
					t.Data = scrubCSS(t.Data)
				}
			}
		}
	}
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type == html.CommentNode && strings.Contains(strings.ToUpper(c.Data), "SLOPIFY_PAGE") {
			n.RemoveChild(c)
			c = next
			continue
		}
		if dropNode(c) {
			n.RemoveChild(c)
			c = next
			continue
		}
		scrub(c)
		c = next
	}
}

func scrubAttrs(n *html.Node) {
	out := n.Attr[:0]
	for _, a := range n.Attr {
		key := strings.ToLower(a.Key)
		if strings.HasPrefix(key, "on") || key == "srcdoc" || key == "srcset" {
			continue
		}
		switch key {
		case "href", "src", "action", "poster", "formaction", "hx-get", "hx-post", "hx-put", "hx-patch", "hx-delete":
			ref, ok := cleanRef(a.Val)
			if !ok {
				continue
			}
			a.Val = ref
		case "hx-push-url":
			push := strings.TrimSpace(a.Val)
			if strings.EqualFold(push, "true") || strings.EqualFold(push, "false") {
				a.Val = strings.ToLower(push)
				break
			}
			ref, ok := cleanRef(push)
			if !ok {
				continue
			}
			a.Val = ref
		case "hx-trigger":
			next, keep := dropAutoTrigger(a.Val)
			if !keep {
				continue
			}
			a.Val = next
		case "style":
			if badStyle(a.Val) {
				continue
			}
		}
		out = append(out, a)
	}
	n.Attr = out
}

// dropAutoTrigger removes load and every. A swapped element that still has
// hx-trigger="load" requests itself again.
func dropAutoTrigger(val string) (string, bool) {
	var kept []string
	for _, part := range strings.Split(val, ",") {
		spec := strings.TrimSpace(part)
		if spec == "" {
			continue
		}
		event := spec
		if i := strings.IndexAny(spec, " \t"); i >= 0 {
			event = spec[:i]
		}
		if strings.EqualFold(event, "load") || strings.EqualFold(event, "every") {
			continue
		}
		kept = append(kept, spec)
	}
	if len(kept) == 0 {
		return "", false
	}
	return strings.Join(kept, ", "), true
}

func cleanRef(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if v == "" || strings.HasPrefix(v, "#") {
		return v, true
	}
	if strings.Contains(v, ":") || strings.Contains(v, `\`) || strings.HasPrefix(v, "//") {
		return "", false
	}
	if !strings.HasPrefix(v, "/") {
		v = "/" + v
	}
	if len(v) > 200 {
		return "", false
	}
	return v, true
}

func badStyle(v string) bool {
	l := strings.ToLower(strings.TrimSpace(v))
	return strings.Contains(l, "url(") || strings.Contains(l, "expression") || strings.Contains(l, "javascript") || strings.Contains(l, "@import")
}

func scrubCSS(css string) string {
	lower := strings.ToLower(css)
	var b strings.Builder
	i := 0
	for i < len(css) {
		rest := lower[i:]
		imp := strings.Index(rest, "@import")
		urlAt := strings.Index(rest, "url(")
		if imp < 0 && urlAt < 0 {
			b.WriteString(css[i:])
			break
		}
		if imp >= 0 && (urlAt < 0 || imp < urlAt) {
			b.WriteString(css[i : i+imp])
			semi := strings.Index(css[i+imp:], ";")
			if semi < 0 {
				break
			}
			i += imp + semi + 1
			continue
		}
		end := strings.Index(css[i+urlAt:], ")")
		if end < 0 {
			b.WriteString(css[i:])
			break
		}
		b.WriteString(css[i : i+urlAt])
		raw := css[i+urlAt+4 : i+urlAt+end]
		val := strings.Trim(raw, " \t\n\r\"'")
		if strings.Contains(val, ":") || strings.Contains(val, "//") {
			b.WriteString("none")
		} else {
			b.WriteString(css[i+urlAt : i+urlAt+end+1])
		}
		i += urlAt + end + 1
	}
	return strings.ReplaceAll(b.String(), "</", "")
}

func findPage(nodes []*html.Node) *html.Node {
	var byID, byMain *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if byID == nil && getAttr(n, "id") == "page" {
				byID = n
			}
			if byMain == nil && n.Data == "main" {
				byMain = n
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	if byID != nil {
		return byID
	}
	return byMain
}

func siteName(nodes []*html.Node, page *html.Node) string {
	if s := headingText(nodes, page, false); s != "" {
		return s
	}
	if s := labelText(nodes, page, false); s != "" {
		return s
	}
	if page == nil {
		return ""
	}
	if s := headingText([]*html.Node{page}, nil, true); s != "" {
		return s
	}
	return labelText([]*html.Node{page}, nil, true)
}

func headingText(nodes []*html.Node, skip *html.Node, inside bool) string {
	for _, tag := range []string{"h1", "h2", "h3"} {
		if s := oneLine(firstText(nodes, skip, tag, inside), 80); s != "" {
			return s
		}
	}
	return ""
}

// labelText is the navbar brand or screen title when the model did not use a heading.
func labelText(nodes []*html.Node, skip *html.Node, inside bool) string {
	roots := nodes
	if inside && len(nodes) == 1 {
		var kids []*html.Node
		for c := nodes[0].FirstChild; c != nil; c = c.NextSibling {
			kids = append(kids, c)
		}
		roots = kids
		skip = nil
	}
	if s := oneLine(findLabel(roots, skip, true), 80); s != "" {
		return s
	}
	return oneLine(findLabel(roots, skip, false), 80)
}

func findLabel(nodes []*html.Node, skip *html.Node, titleSize bool) string {
	var found string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != "" || n == nil || n == skip {
			return
		}
		if n.Type == html.ElementNode {
			label := ""
			if titleSize && titleSized(n) {
				label = textContent(n)
			} else if !titleSize {
				label = directText(n)
			}
			if s := strings.TrimSpace(label); s != "" && !chromeLabel(s) {
				found = s
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return found
}

func directText(n *html.Node) string {
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
	}
	return b.String()
}

func titleSized(n *html.Node) bool {
	class := " " + getAttr(n, "class") + " "
	for _, size := range []string{"text-xl", "text-2xl", "text-3xl", "text-4xl"} {
		if strings.Contains(class, " "+size+" ") || strings.Contains(class, " "+size+"/") || strings.Contains(class, ":"+size+" ") || strings.Contains(class, ":"+size+"/") {
			return true
		}
	}
	return false
}

func chromeLabel(s string) bool {
	s = strings.ToLower(strings.Join(strings.Fields(s), " "))
	if len([]rune(s)) < 2 {
		return true
	}
	switch s {
	case "home", "play", "stats", "how to", "how-to", "new site", "menu", "login", "log in", "sign in", "sign up", "signup", "register", "back", "close":
		return true
	}
	return false
}

func headingInside(page *html.Node) string {
	for _, tag := range []string{"h1", "h2", "h3"} {
		if s := firstText([]*html.Node{page}, nil, tag, true); s != "" {
			return oneLine(s, 120)
		}
	}
	return ""
}

func firstText(nodes []*html.Node, skip *html.Node, tag string, inside bool) string {
	var found string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != "" || n == nil || n == skip {
			return
		}
		if n.Type == html.ElementNode && n.Data == tag {
			found = textContent(n)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	if inside && len(nodes) == 1 {
		for c := nodes[0].FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		return found
	}
	for _, n := range nodes {
		walk(n)
	}
	return found
}

func skeleton(nodes []*html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return cleanSkeleton(b.String())
}

func cleanSkeleton(value string) string {
	var b strings.Builder
	for _, field := range strings.Fields(strings.ToLower(value)) {
		if !tagToken(field) {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(field)
		if b.Len() >= 180 {
			break
		}
	}
	return strings.TrimSpace(b.String())
}

func tagToken(s string) bool {
	if s == "" || len(s) > 30 || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func replaceNode(nodes []*html.Node, old, neu *html.Node) []*html.Node {
	if old.Parent != nil {
		old.Parent.InsertBefore(neu, old)
		old.Parent.RemoveChild(old)
		return nodes
	}
	out := make([]*html.Node, 0, len(nodes))
	for _, n := range nodes {
		if n == old {
			out = append(out, neu)
			continue
		}
		out = append(out, n)
	}
	return out
}

func outerHTML(n *html.Node) (string, error) {
	return renderNodes([]*html.Node{n})
}

func renderNodes(nodes []*html.Node) (string, error) {
	var b strings.Builder
	for _, n := range nodes {
		if err := html.Render(&b, n); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func oneLine(s string, n int) string {
	return clip(strings.Join(strings.Fields(s), " "), n)
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func setAttr(n *html.Node, key, val string) {
	for i := range n.Attr {
		if n.Attr[i].Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

func delAttr(n *html.Node, key string) {
	out := n.Attr[:0]
	for _, a := range n.Attr {
		if a.Key != key {
			out = append(out, a)
		}
	}
	n.Attr = out
}

func titleFromPath(p string) string {
	if p == "/" {
		return "Home"
	}
	parts := strings.Split(strings.Trim(p, "/"), "/")
	words := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ReplaceAll(part, "-", " ")
		part = strings.ReplaceAll(part, "_", " ")
		part = strings.TrimSpace(part)
		if part != "" {
			words = append(words, part)
		}
	}
	if len(words) == 0 {
		return "Home"
	}
	return strings.Join(words, " ")
}

func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[i+1:]
		} else {
			s = ""
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	return strings.TrimSpace(s)
}

// linkedRoutes returns same-site paths referenced by href or hx-get.
func linkedRoutes(raw string) []string {
	nodes, err := parseInput(raw)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for _, key := range []string{"href", "hx-get"} {
				ref, ok := cleanRef(getAttr(n, key))
				if !ok {
					continue
				}
				route, ok := CleanRoute(ref)
				if !ok || seen[route] {
					continue
				}
				seen[route] = true
				out = append(out, route)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return out
}

func clip(s string, n int) string {
	s = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s))
	rs := []rune(s)
	if len(rs) > n {
		return strings.TrimSpace(string(rs[:n]))
	}
	return s
}

// CleanRoute reports the canonical path for a generated page.
func CleanRoute(p string) (string, bool) {
	if p == "" || !strings.HasPrefix(p, "/") || strings.ContainsRune(p, 0) || len(p) > 200 {
		return "", false
	}
	c := path.Clean(p)
	if c == "/health" || c == "/static" || strings.HasPrefix(c, "/static/") || c == "/new-site" || c == "/state" || strings.HasPrefix(c, "/state/") {
		return "", false
	}
	return c, true
}
