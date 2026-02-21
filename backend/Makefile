.PHONY: run seed-scraped help

help:
	@echo "Available commands:"
	@echo "  make run            - Run the backend server"
	@echo "  make seed-scraped   - Seed scraped data from JSON file"

run:
	go run main.go

seed-scraped:
	go run scripts/seed_scraped.go
