package site

import (
	"fmt"
	"strings"
)

// A brief is a visual direction rolled per site. The model still writes the
// frame. Consecutive sites are rolled off the previous brief so New site
// cannot keep the same color, type, composition, and density.
type brief struct {
	p, t, c, d, k int
}

func (b brief) code() string {
	return fmt.Sprintf("%d.%d.%d.%d.%d", b.p, b.t, b.c, b.d, b.k)
}

func parseBrief(code string) (brief, bool) {
	var b brief
	n, err := fmt.Sscanf(code, "%d.%d.%d.%d.%d", &b.p, &b.t, &b.c, &b.d, &b.k)
	if err != nil || n != 5 || code != b.code() {
		return brief{}, false
	}
	if b.p < 0 || b.p >= len(briefColors) || b.t < 0 || b.t >= len(briefTypes) || b.c < 0 || b.c >= len(briefCompositions) || b.d < 0 || b.d >= len(briefDensity) || b.k < 0 || b.k >= len(topics) {
		return brief{}, false
	}
	return b, true
}

func rollBrief(nonce, avoidCode string) brief {
	n := 0
	for _, r := range nonce {
		n = n*131 + int(r)
	}
	if n < 0 {
		n = -n
	}
	b := brief{
		p: n % len(briefColors),
		t: (n / 17) % len(briefTypes),
		c: (n / 29) % len(briefCompositions),
		d: (n / 43) % len(briefDensity),
		k: (n / 61) % len(topics),
	}
	prev, ok := parseBrief(avoidCode)
	if !ok {
		return b
	}
	b.p = shift(b.p, prev.p, len(briefColors), n)
	b.t = shift(b.t, prev.t, len(briefTypes), n>>3)
	b.c = shift(b.c, prev.c, len(briefCompositions), n>>6)
	b.d = shift(b.d, prev.d, len(briefDensity), n>>9)
	b.k = shift(b.k, prev.k, len(topics), n>>12)
	return b
}

func shift(choice, prev, n, salt int) int {
	if n <= 1 {
		return 0
	}
	if choice == prev {
		hop := 1 + salt%(n-1)
		if hop < 0 {
			hop = -hop
		}
		choice = (prev + hop) % n
	}
	if choice == prev {
		choice = (prev + 1) % n
	}
	return choice
}

func usesBriefTheme(shell string, b brief) bool {
	theme := briefColors[b.p]
	return strings.Contains(shell, `data-theme="`+theme+`"`)
}

func driftedTopic(name, html, topic string) bool {
	if coastTopic(topic) {
		return false
	}
	bad := map[string]bool{
		"lighthouse": true, "fog": true, "foghorn": true, "foggy": true,
		"harbor": true, "harbour": true, "maritime": true, "buoy": true,
		"headland": true, "shipwreck": true,
	}
	text := strings.ToLower(name + " " + html)
	for _, word := range strings.FieldsFunc(text, func(r rune) bool {
		return r < 'a' || r > 'z'
	}) {
		if bad[word] {
			return true
		}
	}
	return false
}

func coastTopic(topic string) bool {
	for _, word := range strings.Fields(strings.ToLower(topic)) {
		switch word {
		case "lighthouse", "fog", "harbor", "harbour", "maritime", "buoy", "headland":
			return true
		}
	}
	return false
}

var topics = []string{
	"a tip splitter",
	"a unit converter",
	"a pomodoro timer",
	"a habit tracker",
	"a grocery list",
	"a recipe box",
	"a study card list",
	"a workout log",
	"a bill splitter",
	"a password generator",
	"a movie watchlist",
	"a reading list",
	"a simple budget",
	"a packing list",
	"a name picker",
	"a breathing timer",
	"a daily journal",
	"a pace calculator",
	"a water intake tracker",
	"a gift list",
	"a sleep log",
	"a meal planner",
	"a chore list",
	"a meeting note",
	"a subscription tracker",
	"a birthday list",
	"a savings goal",
	"a time zone converter",
	"a note list",
	"a weekly planner",
	"a fuel cost split",
	"a recipe scaler",
	"a date countdown",
	"a contact list",
	"a plant watering schedule",
	"a checklist",
}

var briefColors = []string{
	"light",
	"dark",
	"nord",
	"cupcake",
	"dracula",
	"luxury",
	"lemonade",
	"sunset",
	"aqua",
	"forest",
	"dim",
	"retro",
}

var briefTypes = []string{
	"font-sans. The screen title is text-2xl. Labels and rows use text-sm.",
	"font-mono. Put font-mono on the root. Data and labels use text-sm.",
	"font-sans. The screen title is text-xl font-semibold. Controls use text-sm.",
	"font-sans for the workspace and font-mono for ids, data, and kbd.",
	"font-serif for the screen title only, at text-3xl. Everything else is font-sans text-sm.",
	"font-mono text-sm, with divider components between rows.",
	"font-sans font-medium. The screen title is text-3xl. Rows use text-sm.",
	"font-sans. No display type. Titles are text-lg. Data is text-sm.",
}

var briefCompositions = []string{
	"App shell. From md up, a vertical menu on the left and the workspace on the right. On a phone, a compact navbar above the workspace.",
	"Top navbar with the app name and screen links. The workspace fills the rest.",
	"A dense workspace: a toolbar of buttons, then one table or list that is the screen.",
	"Split pane from md up: a list of records, then the open record. On a phone, stack the list above the record.",
	"A few columns of cards, like a queue. Columns stack on a phone.",
	"Tabs across the top of the workspace. The open tab is the screen.",
	"A tool: one fieldset for the task, and the result or list beside it from md up.",
	"A short toolbar, the working surface, then a thin status strip.",
	"A master list. Each row is one record and links to another screen.",
	"A narrow menu of screen names beside an inset workspace panel.",
	"A filter or input bar, then results that fill the rest of the screen.",
	"One record open in the workspace, with the other screens only in the navbar.",
}

var briefDensity = []string{
	"Compact. Tight toolbars and rows, like a tool someone uses every day.",
	"Comfortable. Clear gaps between controls, still a workspace.",
	"Dense. Many rows or fields visible at once.",
	"Focused. One primary task and few competing controls.",
}
