package v1

import (
	"context"
	"log/slog"
	"net/http"

	handle "github.com/DillonEnge/http-handler-go"
	"github.com/DillonEnge/slopify/internal/site"
)

func HandleState(sessions *site.Store, gen site.Generator) handle.HandlerFuncWithError {
	return func(w http.ResponseWriter, r *http.Request) *handle.HandlerError {
		op, resource, id, action, ok := site.ParseStatePath(r.Method, r.PathValue("path"))
		if !ok {
			http.NotFound(w, r)
			return nil
		}
		sess := sessions.Session(w, r)
		ctx, cancel := context.WithTimeout(r.Context(), site.GenerateTimeout)
		defer cancel()
		call := site.HTMXFrom(r)
		call.Action = action
		frag, err := sess.Apply(ctx, gen, op, resource, id, site.FieldsFrom(r), call)
		if err != nil {
			slog.Error("state", "op", op, "resource", resource, "id", id, "err", err)
			http.Error(w, site.ErrorText(err), http.StatusBadGateway)
			return nil
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if _, err := w.Write([]byte(frag)); err != nil {
			return &handle.HandlerError{Status: http.StatusInternalServerError, Err: err}
		}
		return nil
	}
}
