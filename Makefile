.PHONY: up down logs test test-unit build fmt vet

up:
	docker compose up -d --build app

down:
	docker compose down -v

logs:
	docker compose logs -f app

test:
	docker compose --profile test run --rm tests

test-unit:
	go test -count=1 ./...

build:
	go build -o bin/server ./cmd/server

fmt:
	gofmt -w .

vet:
	go vet ./...
