.PHONY: run build test lint clean migrate-up

# Backend commands
run:
	cd backend && go run cmd/synapse/main.go

build:
	cd backend && go build -o bin/synapse cmd/synapse/main.go

test:
	cd backend && go test -v -race ./...

lint:
	cd backend && golangci-lint run ./...

bench:
	cd backend && go test -bench=. -benchmem ./internal/engine/...

# Frontend commands
dev-frontend:
	cd frontend && npm run dev

build-frontend:
	cd frontend && npm run build

# Docker
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

# Database
migrate-up:
	cd backend && SYNAPSE_DATABASE_URL="postgresql://synapse:synapse@localhost:5432/synapse?sslmode=disable" go run ./cmd/migrate up

clean:
	cd backend && rm -rf bin/
	cd frontend && rm -rf dist/ node_modules/
