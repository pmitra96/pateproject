.PHONY: build run prod-run seed-scraped help

BINARY_NAME=main

help:
	@echo "Available commands:"
	@echo "  make build          - Build the Go binary"
	@echo "  make run            - Run the backend server (dev)"
	@echo "  make prod-run       - Execute the built binary"
	@echo "  make seed-scraped   - Seed scraped data from JSON file"

build:
	CGO_ENABLED=1 go build -o $(BINARY_NAME) ./cmd/server/main.go

run:
	go run ./cmd/server/main.go

prod-run:
	./$(BINARY_NAME)

seed-scraped:
	go run scripts/seed_scraped.go
