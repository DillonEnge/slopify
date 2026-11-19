# Slopify

One browser session gets one generated site. The model writes the HTML. The server keeps the frame, the pages, and the records in memory.

`GET /health` returns 204. The process listens on `PORT`, which defaults to `7600`.

## Run

Put the key in `.env`. That file stays untracked.

```
ANTHROPIC_API_KEY=...
PORT=7600
```

```
docker compose up --build
```

`make dev` runs templ, Tailwind, and Air together. `make test` runs `go test ./...`. `make regen` rewrites `page_templ.go` and `output.css` without the watchers.

## A page is two responses

A document navigation that is not already cached returns the loader and does not call the model. `#boot` then requests the same path with `hx-trigger="load"`. That second request carries `HX-Request` and is the one that waits on generation.

```
GET /inbox          no HX-Request, pages["/inbox"] missing
                    -> template.Boot("/inbox")

GET /inbox          HX-Request: true
                    -> Session.View
                    -> template.Document, or template.Fragment when HX-Boosted
```

`Ready` is true only when the session already has a site and a cached page for that path. A cached document navigation skips the loader.

The loader is a queue ticket. Position, hold, and node are a hash of the path. `/` is a deeper ticket than any other path. A script counts the ticket down and stalls on `SERVING`. The real wait is the model call behind it. A generation failure before any site exists renders `template.Unavailable` as 200, because a 502 would leave `#boot` on the page. A boosted failure still writes 502.

## One frame, then pages

`/` is the only call with `home=true`. Claude Sonnet 5, at medium effort, writes the chrome and the first screen as one fragment. `frameFrom` lifts `<main id="page">` out and leaves `<!--SLOPIFY_PAGE-->` in the shell.

```
Site.Shell     chrome + <!--SLOPIFY_PAGE-->
Site.HomeHTML  the first <main id="page">
pages["/"]     that same main
```

`Site.Compose` puts a page back into the slot. Every later path calls `GenerateRoute` with `home=false`: Claude Haiku 5.5, at low effort, one `<main id="page">`. The prompt includes the composed current page, so the new main keeps the ids the previous controls swap.

Both calls use adaptive thinking, a 16000 token cap, and an ephemeral cache on the system prompt. A home frame that misses `data-theme`, or that drifts off the assigned topic, is retried once. A route that does not parse as a main is retried once. A failed call is not stored.

The document wraps the composed frame in `#shell`:

```html
<div id="shell" style="display:contents"
     hx-boost="true" hx-target="#page" hx-select="#page"
     hx-swap="outerHTML show:window:top" hx-history="false">
```

Boost lives on `#shell` because HTMX 2.0.2 replaces the children of `body` and does not apply attributes from `html` or `body`. `hx-history="false"` makes the back button refetch. A boosted click receives `template.Fragment`, which is the new main. The loader's follow-up is not boosted, so it receives the full document and replaces `#boot`.

`New site` is an anchor outside `main`, with `hx-boost="false"` and no `hx-get`. `GET /new-site` drops the session and redirects to `/`.

## A session is the cache

`slopify_session` is an HttpOnly cookie, seven days, and a key into an in-memory map. Nothing is written to disk. A restart, or a cookie whose id is gone, starts a new session.

`claimLocked` is the join. The first caller for a path owns a `routeCall` and starts the job. Everyone else waits on the same `done` channel. The job uses `context.Background` with a three-minute timeout, so it keeps running after the HTTP request that started it gives up.

After a page is served, prefetch starts one goroutine per same-site `href` or `hx-get` on the composed page. It does not walk past that page. `/state`, `/new-site`, `/health`, and `/static` are not page routes, so they are not prefetched.

## The brief is the dice

`rollBrief` folds the session id into indexes for a DaisyUI theme, a type ramp, a composition, a density, and a topic. The home prompt assigns that topic and one form: a web app, a single tool, a log, or a lookup.

`Forget` copies `Site.Brief` into `slopify_avoid` for ten minutes. The next home roll shifts any index that matches the previous brief, and the prompt names the forbidden color, composition, and topic.

## `/state` is the data

Generated pages are cached on first render. Records are not. A control that changes data targets `/state/...` and swaps one element. The page cache stays as it was.

| Request | Op |
| --- | --- |
| `GET /state/items` | list |
| `GET /state/items/{id}` | read |
| `POST /state/items` | create |
| `PUT` or `PATCH /state/items/{id}` | update |
| `DELETE /state/items/{id}` | delete |
| `GET /state/rounds/draw/vowel` | action |

`Apply` mutates `session.records` first, then asks Haiku for one element whose id is `HX-Target`, or the resource name when that header is empty. The prompt includes the operation, the records after the change, the HTMX headers, and the cached page for the `Referer` path. The reply is scrubbed. `hx-trigger="load"` and `hx-trigger="every ..."` are removed, so a swapped element does not request itself.

The copy sent to the browser is not the stored page. `armStateRefresh` adds `hx-trigger="load"` to a region that reads itself: an `hx-get` of a list or a record, an `hx-target` of `this` or its own id, and not a button or an action. Navigating back therefore hits `/state` again. The stored HTML does not carry that trigger.

## Where it lives

| Package | Role |
| --- | --- |
| `cmd` | `PORT`, default `7600` |
| `internal/service` | mux: health, static, `/new-site`, `/state`, pages |
| `internal/api/v1` | loader gate, document versus fragment, state write |
| `internal/site` | session, frame split, brief, records |
| `internal/textgen` | `site.Generator` over the Anthropic Messages API |
| `internal/template` | loader, document, fragment; CSS is `go:embed` |

Static files are served with `middleware.NoCache`. Tailwind and DaisyUI are compiled into `internal/template/static/output.css`. The generated frame is told to use those classes, and to leave out a `<style>` element and hex colors.
