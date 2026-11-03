.PHONY: build frontend test dev clean

web/node_modules/.package-lock.json: web/package.json web/package-lock.json
	cd web && npm ci

frontend: web/node_modules/.package-lock.json
	cd web && npm run build

build: frontend
	mkdir -p bin
	go build -trimpath -o bin/codevisual ./cmd/codevisual

test: web/node_modules/.package-lock.json
	go test ./...
	cd web && npm test

dev: build
	@./bin/codevisual analyze . --no-browser & backend=$$!; trap 'kill $$backend 2>/dev/null || true' EXIT INT TERM; cd web && npm run dev

clean:
	go clean
