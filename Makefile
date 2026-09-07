.PHONY: build run db-load test lint tidy

build:
	go build -o ./bin/api ./cmd/api

run: build
	./bin/api

db-load:
	psql "$$DATABASE_URL" -f db/schema.sql
	psql "$$DATABASE_URL" -f db/seed.sql

test:
	go test ./...

lint:
	golangci-lint run

tidy:
	go mod tidy
