.PHONY: dev-backend dev-frontend dev build-backend build-frontend build test clean

dev-backend:
	go run ./cmd/glyphdeck

dev-frontend:
	cd web && npm run dev

dev:
	@echo "Run backend and frontend in separate terminals:"
	@echo "  make dev-backend"
	@echo "  make dev-frontend"

build-backend:
	go build -o bin/glyphdeck.exe ./cmd/glyphdeck

build-frontend:
	cd web && npm run build

build: build-backend build-frontend

test:
	go test ./...

clean:
	rm -rf bin/ web/dist/
