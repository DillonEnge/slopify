package template

import "github.com/DillonEnge/slopify/internal/site"

func windowTitle(s site.Site, p site.Page) string {
	if p.Title == "" {
		return s.Name
	}
	if s.Name == "" {
		return p.Title
	}
	return p.Title + " · " + s.Name
}
