package site

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"golang.org/x/net/html"
)

const stateInstructions = "You return one HTML element for an HTMX swap. Its id is the target. Use DaisyUI and Tailwind classes. Keep what the current element already shows, then apply the action. If the element and the records are empty, show a fresh start. Do not set hx-trigger=\"load\" or hx-trigger=\"every ...\". Reply with the element and nothing else."

// HTMXCall is the request the browser's HTMX control sent.
type HTMXCall struct {
	Target      string
	Trigger     string
	TriggerName string
	CurrentURL  string
	Prompt      string
	Action      string
	Referer     string
}

type record struct {
	ID     string
	Fields map[string]string
}

// Apply changes one resource in this session and returns the HTML element that
// replaces the calling HTMX target.
func (s *Session) Apply(ctx context.Context, g Generator, op, resource, id string, fields map[string]string, call HTMXCall) (string, error) {
	if !validSlug(resource) {
		return "", fmt.Errorf("bad resource")
	}
	if id != "" && !validSlug(id) {
		return "", fmt.Errorf("bad id")
	}
	fields = cloneFields(fields)
	if call.Action != "" {
		if _, ok := fields["action"]; !ok {
			fields["action"] = call.Action
		}
	}
	s.mu.Lock()
	if s.records == nil {
		s.records = map[string][]record{}
	}
	list, missing := mutate(s.records[resource], op, id, fields)
	if op == "create" || op == "act" || op == "update" || op == "delete" {
		s.records[resource] = list
	}
	snap := cloneRecords(list)
	name, class, view := "", "", s.pageByReferer(call.Referer)
	if s.site != nil {
		name, class = s.site.Name, s.site.PageClass
	}
	s.mu.Unlock()

	target := strings.TrimPrefix(strings.TrimSpace(call.Target), "#")
	if target == "" {
		target = resource
	}
	call.Target = target
	current := elementByID(view, target)
	if current == "" {
		current = view
	}
	input := statePrompt(op, resource, id, missing, snap, fields, call, name, class, current)
	text, err := g.Complete(ctx, false, stateInstructions, input)
	if err != nil {
		return "", err
	}
	frag, ferr := fragmentFrom(text, target)
	if ferr != nil {
		text2, err2 := g.Complete(ctx, false, stateInstructions, input+"\n\nReply with only the HTML element.")
		if err2 != nil {
			return "", err2
		}
		frag, ferr = fragmentFrom(text2, target)
	}
	return frag, ferr
}

// armStateRefresh makes a cached page fetch its state regions again. The page
// store keeps the first render, so navigating back would otherwise show that
// HTML and never call /state. A region that reads itself gets hx-trigger="load".
// The state response still strips load, so the swap does not request itself.
func armStateRefresh(raw string) string {
	nodes, err := parseInput(raw)
	if err != nil {
		return raw
	}
	changed := false
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && stateView(n) && addLoadTrigger(n) {
			changed = true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	if !changed {
		return raw
	}
	out, err := renderNodes(nodes)
	if err != nil {
		return raw
	}
	return out
}

// stateView is a region whose own hx-get reads a list or one record.
// Buttons and actions stay put so a page show does not draw a vowel or submit a form.
func stateView(n *html.Node) bool {
	switch n.Data {
	case "a", "button", "input", "select", "textarea", "label", "summary":
		return false
	}
	if strings.EqualFold(getAttr(n, "role"), "button") {
		return false
	}
	ref, ok := cleanRef(getAttr(n, "hx-get"))
	if !ok {
		return false
	}
	if i := strings.IndexByte(ref, '?'); i >= 0 {
		ref = ref[:i]
	}
	if !strings.HasPrefix(ref, "/state/") {
		return false
	}
	op, _, _, _, ok := ParseStatePath(http.MethodGet, strings.TrimPrefix(ref, "/state/"))
	if !ok || (op != "list" && op != "read") {
		return false
	}
	return targetsSelf(n)
}

func targetsSelf(n *html.Node) bool {
	target := strings.TrimSpace(getAttr(n, "hx-target"))
	if target == "" || strings.EqualFold(target, "this") {
		return true
	}
	id := getAttr(n, "id")
	return id != "" && (target == "#"+id || target == id)
}

func addLoadTrigger(n *html.Node) bool {
	cur := strings.TrimSpace(getAttr(n, "hx-trigger"))
	if cur == "" {
		setAttr(n, "hx-trigger", "load")
		return true
	}
	if triggerHas(cur, "load") {
		return false
	}
	setAttr(n, "hx-trigger", "load, "+cur)
	return true
}

func triggerHas(val, event string) bool {
	for _, part := range strings.Split(val, ",") {
		spec := strings.TrimSpace(part)
		if spec == "" {
			continue
		}
		name := spec
		if i := strings.IndexAny(spec, " \t"); i >= 0 {
			name = spec[:i]
		}
		if strings.EqualFold(name, event) {
			return true
		}
	}
	return false
}

// pageByReferer returns the cached page for the Referer path. The caller holds s.mu.
func (s *Session) pageByReferer(referer string) string {
	route, ok := routeFromReferer(referer)
	if !ok || s.pages == nil {
		return ""
	}
	return s.pages[route].HTML
}

func routeFromReferer(referer string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(referer))
	if err != nil {
		return "", false
	}
	return CleanRoute(u.Path)
}

// ParseStatePath reads /state/{resource} and /state/{resource}/{action...}.
// A GET with two or more extra segments is an action, such as rounds/draw/vowel.
func ParseStatePath(method, path string) (op, resource, id, action string, ok bool) {
	path = strings.Trim(path, "/")
	if path == "" {
		return "", "", "", "", false
	}
	parts := strings.Split(path, "/")
	if len(parts) > 6 {
		return "", "", "", "", false
	}
	for _, part := range parts {
		if !validSlug(part) {
			return "", "", "", "", false
		}
	}
	resource = parts[0]
	rest := parts[1:]
	switch method {
	case http.MethodGet:
		if len(rest) == 0 {
			return "list", resource, "", "", true
		}
		if len(rest) == 1 {
			return "read", resource, rest[0], "", true
		}
		return "act", resource, "", strings.Join(rest, "/"), true
	case http.MethodPost:
		return "create", resource, "", strings.Join(rest, "/"), true
	case http.MethodPut, http.MethodPatch:
		if len(rest) == 0 {
			return "", "", "", "", false
		}
		return "update", resource, rest[0], strings.Join(rest[1:], "/"), true
	case http.MethodDelete:
		if len(rest) == 0 {
			return "", "", "", "", false
		}
		return "delete", resource, rest[0], strings.Join(rest[1:], "/"), true
	default:
		return "", "", "", "", false
	}
}

func mutate(list []record, op, id string, fields map[string]string) ([]record, bool) {
	switch op {
	case "create", "act":
		return append(list, record{ID: newRecordID(), Fields: cloneFields(fields)}), false
	case "update":
		for i := range list {
			if list[i].ID != id {
				continue
			}
			next := cloneFields(list[i].Fields)
			maps.Copy(next, fields)
			list[i].Fields = next
			return list, false
		}
		return list, true
	case "delete":
		for i := range list {
			if list[i].ID != id {
				continue
			}
			return append(list[:i], list[i+1:]...), false
		}
		return list, true
	default:
		if op == "read" && id != "" && indexOf(list, id) < 0 {
			return list, true
		}
		return list, false
	}
}

func statePrompt(op, resource, id string, missing bool, list []record, fields map[string]string, call HTMXCall, name, className, current string) string {
	target := strings.TrimPrefix(call.Target, "#")
	if target == "" {
		target = resource
	}
	miss := "no"
	if missing {
		miss = "yes"
	}
	if strings.TrimSpace(current) == "" {
		current = "none"
	}
	action := call.Action
	if action == "" {
		action = "none"
	}
	return fmt.Sprintf(`operation: %s
resource: %s
id: %s
action: %s
missing: %s
site: %s
page class: %s
target: %s
trigger: %s
trigger name: %s
current url: %s
prompt: %s
request fields:
%s
records:
%s
element currently in the target:
%s

Return one element whose id is "%s". It replaces that target. Keep what the current element already shows, and apply the action. If both are empty, show a fresh start. Draw the records above. A missing record stays missing. Do not set hx-trigger="load" or hx-trigger="every ...". Controls that change this resource use /state/%s or /state/%s/{action} with hx-get, hx-post, hx-put, or hx-delete, and set hx-target and hx-select to "%s".`,
		op, resource, id, action, miss, name, className, target, call.Trigger, call.TriggerName, call.CurrentURL, call.Prompt,
		formatFields(fields), formatRecords(list), current, target, resource, resource, target)
}

func fragmentFrom(raw, targetID string) (string, error) {
	nodes, err := parseInput(raw)
	if err != nil {
		return "", err
	}
	var els []*html.Node
	for _, n := range scrubList(nodes) {
		if n.Type == html.ElementNode {
			els = append(els, n)
		}
	}
	if len(els) == 0 {
		return "", fmt.Errorf("state html: empty")
	}
	pick := els[0]
	if targetID != "" {
		if n := findID(els, targetID); n != nil {
			pick = n
		}
		setAttr(pick, "id", targetID)
	}
	return outerHTML(pick)
}

func elementByID(raw, id string) string {
	if strings.TrimSpace(raw) == "" || id == "" {
		return ""
	}
	nodes, err := parseInput(raw)
	if err != nil {
		return ""
	}
	n := findID(nodes, id)
	if n == nil {
		return ""
	}
	out, err := outerHTML(n)
	if err != nil {
		return ""
	}
	return out
}

func findID(nodes []*html.Node, id string) *html.Node {
	var walk func(*html.Node) *html.Node
	walk = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode && getAttr(n, "id") == id {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if found := walk(c); found != nil {
				return found
			}
		}
		return nil
	}
	for _, n := range nodes {
		if found := walk(n); found != nil {
			return found
		}
	}
	return nil
}

func formatRecords(list []record) string {
	if len(list) == 0 {
		return "none"
	}
	var b strings.Builder
	for _, rec := range list {
		b.WriteString(rec.ID)
		for _, k := range slices.Sorted(maps.Keys(rec.Fields)) {
			fmt.Fprintf(&b, " %s=%s", k, rec.Fields[k])
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatFields(fields map[string]string) string {
	if len(fields) == 0 {
		return "none"
	}
	var b strings.Builder
	for _, k := range slices.Sorted(maps.Keys(fields)) {
		fmt.Fprintf(&b, "%s=%s\n", k, fields[k])
	}
	return b.String()
}

func cloneRecords(in []record) []record {
	out := make([]record, len(in))
	for i, rec := range in {
		out[i] = record{ID: rec.ID, Fields: cloneFields(rec.Fields)}
	}
	return out
}

func cloneFields(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	return maps.Clone(in)
}

func indexOf(list []record, id string) int {
	for i := range list {
		if list[i].ID == id {
			return i
		}
	}
	return -1
}

func newRecordID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "record"
	}
	return hex.EncodeToString(b[:])
}

func validSlug(s string) bool {
	if s == "" || len(s) > 40 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
		default:
			return false
		}
	}
	return s[0] != '-' && s[len(s)-1] != '-'
}

// FieldsFrom reads a form into a small set of plain values.
func FieldsFrom(r *http.Request) map[string]string {
	_ = r.ParseForm()
	out := make(map[string]string)
	for key, vals := range r.Form {
		key = strings.TrimSpace(strings.ToLower(key))
		if !validField(key) || len(out) >= 40 {
			continue
		}
		val := ""
		if len(vals) > 0 {
			val = vals[0]
		}
		val = strings.ReplaceAll(val, "\n", " ")
		val = strings.ReplaceAll(val, "\r", " ")
		if len(val) > 500 {
			val = val[:500]
		}
		out[key] = val
	}
	return out
}

func validField(s string) bool {
	if s == "" || len(s) > 40 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// HTMXFrom reads the headers HTMX sends with a request.
func HTMXFrom(r *http.Request) HTMXCall {
	return HTMXCall{
		Target:      r.Header.Get("HX-Target"),
		Trigger:     r.Header.Get("HX-Trigger"),
		TriggerName: r.Header.Get("HX-Trigger-Name"),
		CurrentURL:  r.Header.Get("HX-Current-URL"),
		Prompt:      r.Header.Get("HX-Prompt"),
		Referer:     r.Referer(),
	}
}
