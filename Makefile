.PHONY: fmt lint dev

fmt:
	cd frontend && pnpm run format
	gofmt -w .

lint:
	cd frontend && pnpm run type-check
	go vet ./...

dev:
	wails dev
