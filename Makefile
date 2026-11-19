PORT ?= 7600

templ:
	go tool templ generate -watch -proxy="http://localhost:$(PORT)" -open-browser=false

templ_no_watch:
	go tool templ generate

tailwind:
	pnpm exec tailwindcss -i internal/template/static/input.css -o internal/template/static/output.css --watch=always

tailwind_no_watch:
	pnpm exec tailwindcss -i internal/template/static/input.css -o internal/template/static/output.css --minify

air:
	PORT=$(PORT) go run github.com/air-verse/air@v1.67.4

dev:
	make -j3 templ tailwind air

test:
	go test ./...

lint:
	golangci-lint run

regen: templ_no_watch tailwind_no_watch
