.PHONY: up down restart logs test lint clean

up:
	docker compose up --build -d

down:
	docker compose down -v

restart: down up

logs:
	docker compose logs -f kitchen restaurant

test:
	go test -v -race ./internal/...

lint:
	go vet ./...

clean:
	rm -rf bin/
