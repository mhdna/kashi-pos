.PHONY: fmt lint test dev generate receipt-previews build-windows build-windows-compact

generate:
	sqlc generate

# Receipt look previews for kashi's settings pages. kashi cannot run this
# renderer, so its UI ships the images and a catalog generated from it (see
# TestGenerateReceiptPreviews). Rerun whenever a theme or layout changes, then
# commit the result in kashi.
KASHI_UI ?= ../kashi/ui
receipt-previews:
	rm -rf .receipt-previews && mkdir -p .receipt-previews
	KASHI_RECEIPT_PREVIEW_OUT=$(CURDIR)/.receipt-previews go test -count=1 -run TestGenerateReceiptPreviews .
	mkdir -p $(KASHI_UI)/public/receipt-previews $(KASHI_UI)/src/data
	rm -f $(KASHI_UI)/public/receipt-previews/*.png
	cp .receipt-previews/*.png $(KASHI_UI)/public/receipt-previews/
	cp .receipt-previews/catalog.json $(KASHI_UI)/src/data/receiptThemes.json
	rm -rf .receipt-previews

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

# The next release: the latest tag plus one patch (1.1.65 -> 1.1.66), or
# `make bump PART=minor` / `PART=major`, or `make bump VERSION=1.2.0`.
# Writes it into wails.json and the installer, commits only those two files
# as "chore: release vX" and tags it, so `make publish` then ships it.
# Refuses, tagging nothing, when there are uncommitted changes (publish
# builds what's on disk, so a release must be exactly what's committed), when
# nothing was committed since the last release, or when the code doesn't build.
PART ?= patch
.PHONY: bump
bump:
	@set -e; \
	last=$$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//'); last=$${last:-0.0.0}; \
	if [ -n "$$(git status --porcelain)" ]; then \
		echo "Not bumping: there are uncommitted changes (git status). Commit or stash them first."; exit 1; \
	fi; \
	if git rev-parse -q --verify "refs/tags/v$$last" >/dev/null && \
	   [ -z "$$(git log --format=%s "v$$last..HEAD" | grep -v '^chore: release v')" ]; then \
		echo "Not bumping: nothing committed since v$$last."; exit 1; \
	fi; \
	echo "Checking it builds..."; \
	go build -o /dev/null . || { echo "Not bumping: the code doesn't build."; exit 1; }; \
	next="$(VERSION)"; \
	if [ -z "$$next" ]; then \
		set -- $$(echo "$$last" | tr . " "); ma=$$1; mi=$$2; pa=$$3; \
		case "$(PART)" in \
			major) next=$$((ma + 1)).0.0 ;; \
			minor) next=$$ma.$$((mi + 1)).0 ;; \
			patch) next=$$ma.$$mi.$$((pa + 1)) ;; \
			*) echo "PART must be patch, minor or major"; exit 1 ;; \
		esac; \
	fi; \
	echo "$$next" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "version '$$next' must be X.Y.Z"; exit 1; }; \
	if git rev-parse -q --verify "refs/tags/v$$next" >/dev/null; then echo "v$$next is already tagged"; exit 1; fi; \
	sed -i.bak -E "s/(\"productVersion\": *\")[^\"]*\"/\1$$next.0\"/" wails.json && rm -f wails.json.bak; \
	sed -i.bak -E "s/(INFO_PRODUCTVERSION \")[^\"]*\"/\1$$next.0\"/" build/windows/installer/wails_tools.nsh && rm -f build/windows/installer/wails_tools.nsh.bak; \
	git commit -q -m "chore: release v$$next" -- wails.json build/windows/installer/wails_tools.nsh; \
	git tag "v$$next"; \
	echo "v$$last -> v$$next (committed and tagged; make publish ships it)"

build-windows: set-version
	wails build -platform windows/amd64 -nsis -clean -ldflags "-X main.appVersion=$(APP_VERSION)"

build-windows-compact: set-version
	KASHI_COMPACT=1 VITE_KASHI_COMPACT=1 wails build -platform windows/amd64 -tags compact -nsis -clean -o "GB Cloud Compact.exe" -ldflags "-X main.appVersion=$(APP_VERSION)"

# Remote updates (updater.go). update-keygen runs once: it creates the
# release-signing key in ~/.config/gb-cloud/ (back it up) and writes its
# public half into internal/updatekey -- commit that. publish builds the
# Windows app at the latest tag's version, signs it and uploads it to kashi;
# choose which tills get it on kashi's App Updates page. It needs KASHI_URL
# (the tills' central URL) and KASHI_USER; the password is asked for.
# publish-all (and publish-compact-all) also sets every active till of that
# build to the new release, so the ones online update right away.
#   git tag v1.2.3 && make publish NOTES="what changed"
#   git tag v1.2.3 && make publish-all NOTES="what changed"
# publish-all-force (and publish-compact-all-force) also makes them install it
# right away, even mid-shift, after a one-minute countdown on the till.
#   git tag v1.2.3 && make publish-all-force NOTES="urgent fix"
.PHONY: update-keygen publish publish-compact publish-all publish-compact-all publish-all-force publish-compact-all-force
update-keygen:
	go run ./cmd/publish-release keygen

# Asks which branches get it; BRANCHES=CN,HR (or all) answers up front.
publish: build-windows
	go run ./cmd/publish-release publish -file build/bin/GB-Cloud.exe -version $(APP_VERSION) -flavor standard -notes "$(NOTES)" -branches "$(BRANCHES)"

publish-compact: build-windows-compact
	go run ./cmd/publish-release publish -file "build/bin/GB Cloud Compact.exe" -version $(APP_VERSION) -flavor compact -notes "$(NOTES)" -branches "$(BRANCHES)"

publish-all: build-windows
	go run ./cmd/publish-release publish -file build/bin/GB-Cloud.exe -version $(APP_VERSION) -flavor standard -notes "$(NOTES)" -target-all

publish-compact-all: build-windows-compact
	go run ./cmd/publish-release publish -file "build/bin/GB Cloud Compact.exe" -version $(APP_VERSION) -flavor compact -notes "$(NOTES)" -target-all

publish-all-force: build-windows
	go run ./cmd/publish-release publish -file build/bin/GB-Cloud.exe -version $(APP_VERSION) -flavor standard -notes "$(NOTES)" -target-all -force

publish-compact-all-force: build-windows-compact
	go run ./cmd/publish-release publish -file "build/bin/GB Cloud Compact.exe" -version $(APP_VERSION) -flavor compact -notes "$(NOTES)" -target-all -force
