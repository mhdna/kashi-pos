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

# Version comes from the latest git tag (v1.2.3 or v1.2.3.4 -> 1.2.3[.4]).
# It's shown on the login screen and stamped into wails.json's
# productVersion, which the exe and NSIS installer use (Apps & features,
# upgrade prompt). Windows wants exactly 4 parts, so X.Y.Z is padded to
# X.Y.Z.0 there. Override with `make build-windows APP_VERSION=1.2.3`.
APP_VERSION ?= $(patsubst v%,%,$(shell git describe --tags --abbrev=0 2>/dev/null || echo 0.0.0))

# Wails 2.11's package loader fails under Go 1.27 ("package ... without
# types was imported from ..."), so builds run on the go.mod toolchain.
export GOTOOLCHAIN ?= go1.25.5

WIN_VERSION =$(if $(word 4,$(subst ., ,$(APP_VERSION))),$(APP_VERSION),$(APP_VERSION).0)

.PHONY: set-version
set-version:
	@echo "$(APP_VERSION)" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?$$' || \
		{ echo "version '$(APP_VERSION)' must be X.Y.Z or X.Y.Z.W (tag it like v1.2.3)"; exit 1; }
	sed -i.bak -E 's/("productVersion": *")[^"]*"/\1$(WIN_VERSION)"/' wails.json && rm -f wails.json.bak

build-windows: set-version
	wails build -platform windows/amd64 -nsis -clean -ldflags "-X main.appVersion=$(APP_VERSION)"

build-windows-compact: set-version
	KASHI_COMPACT=1 VITE_KASHI_COMPACT=1 wails build -platform windows/amd64 -tags compact -nsis -clean -o "Kashi Cloud Compact.exe" -ldflags "-X main.appVersion=$(APP_VERSION)"
