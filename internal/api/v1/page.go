package v1

import (
	"context"
	"log/slog"
	"net/http"

	handle "github.com/DillonEnge/http-handler-go"
	"github.com/DillonEnge/slopify/internal/site"
	"github.com/DillonEnge/slopify/internal/template"
)

func HandlePage(sessions *site.Store, gen site.Generator) handle.HandlerFuncWithError {
	return func(w http.ResponseWriter, r *http.Request) *handle.HandlerError {
		route, ok := site.CleanRoute(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return nil
		}
		if route != r.URL.Path {
			http.Redirect(w, r, route, http.StatusTemporaryRedirect)
			return nil
		}

		sess := sessions.Session(w, r)
		// A document navigation must return before generation. The loader's
		// load request carries HX-Request and is swapped in place. Boosted
		// navigations already have the shell, so they receive a fragment.
		boosted := r.Header.Get("HX-Boosted") == "true"
		if r.Header.Get("HX-Request") != "true" && !sess.Ready(route) {
			if rerr := template.Boot(route).Render(r.Context(), w); rerr != nil {
				return &handle.HandlerError{Status: http.StatusInternalServerError, Err: rerr}
			}
			return nil
		}

		ctx, cancel := context.WithTimeout(r.Context(), site.GenerateTimeout)
		defer cancel()

		view, page, err := sess.View(ctx, gen, route)
		if err != nil {
			slog.Error("generate view", "path", route, "err", err)
			if view.Name == "" {
				// 502 is not swapped, which would leave the loader in place.
				if boosted {
					w.WriteHeader(http.StatusBadGateway)
				}
				if rerr := template.Unavailable(site.ErrorText(err)).Render(r.Context(), w); rerr != nil {
					return &handle.HandlerError{Status: http.StatusInternalServerError, Err: rerr}
				}
				return nil
			}
			page = site.Page{
				Path:  route,
				Title: "Unavailable",
				HTML:  site.PlainPage(view.PageClass, "Unavailable", site.ErrorText(err)),
			}
		}

		var rerr error
		if boosted {
			rerr = template.Fragment(view, page).Render(r.Context(), w)
		} else {
			rerr = template.Document(view, page).Render(r.Context(), w)
		}
		if rerr != nil {
			return &handle.HandlerError{Status: http.StatusInternalServerError, Err: rerr}
		}
		return nil
	}
}

func HandleNewSite(sessions *site.Store) handle.HandlerFuncWithError {
	return func(w http.ResponseWriter, r *http.Request) *handle.HandlerError {
		sessions.Forget(w, r)
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", "/")
			w.WriteHeader(http.StatusNoContent)
			return nil
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return nil
	}
}
