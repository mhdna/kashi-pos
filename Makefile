.PHONY: fmt lint test dev build-windows build-windows-compact

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

build-windows:
	wails build -platform windows/amd64 -clean

build-windows-compact:
	KASHI_COMPACT=1 VITE_KASHI_COMPACT=1 wails build -platform windows/amd64 -tags compact -clean -o kashi-pos-compact.exe
