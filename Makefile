.PHONY: fmt lint test dev generate build-windows build-windows-compact

generate:
	sqlc generate

fmt:
	cd frontend && pnpm run format
	gofmt -w .

lint:
	cd frontend && pnpm run type-check
	go vet ./...
	golangci-lint run ./...

test:
	go test ./...
	cd frontend && pnpm run type-check && pnpm run build

dev:
	wails dev

APP_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build-windows:
	wails build -platform windows/amd64 -nsis -clean -ldflags "-X main.appVersion=$(APP_VERSION)"

build-windows-compact:
	KASHI_COMPACT=1 VITE_KASHI_COMPACT=1 wails build -platform windows/amd64 -tags compact -nsis -clean -o "Kashi Cloud Compact.exe" -ldflags "-X main.appVersion=$(APP_VERSION)"
