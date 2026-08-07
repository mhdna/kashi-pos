.PHONY: fmt lint test dev build-windows build-windows-compact build-central

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
	KASHI_COMPACT=1 VITE_KASHI_COMPACT=1 wails build -platform windows/amd64 -tags compact -clean -o "Kashi Cloud Compact.exe"

# Central builds receive their private key only through the build environment.
# The key is compiled into the backend and is never exposed to the frontend.
build-central:
	VITE_KASHI_BUILD_PROFILE=central VITE_KASHI_ENABLED_PAGES=import-data,inventory wails build -clean -ldflags "-X main.kashiBuildProfile=central -X main.kashiCentralPrivateKey=$$KASHI_CENTRAL_PRIVATE_KEY"
