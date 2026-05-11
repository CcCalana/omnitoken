.PHONY: help up down logs test lint vet format docker-build clean

help:
	@echo "OmniToken commands:"
	@echo "  make up            Start local Postgres, Redis, NATS, gateway, and admin"
	@echo "  make down          Stop local services"
	@echo "  make logs          Follow compose logs"
	@echo "  make test          Run Go tests with race detector"
	@echo "  make lint          Run Go vet"
	@echo "  make format        Format Go code"
	@echo "  make docker-build  Build gateway, admin, and migrate container images"

up:
	docker build -f deploy/Dockerfile.gateway -t omnitoken-gateway:local .
	docker build -f deploy/Dockerfile.admin -t omnitoken-admin:local .
	docker build -f deploy/Dockerfile.migrate -t omnitoken-migrate:local .
	docker compose -f deploy/docker-compose.yml up -d --no-build

down:
	docker compose -f deploy/docker-compose.yml down

logs:
	docker compose -f deploy/docker-compose.yml logs -f

test:
	go test -race ./...

lint vet:
	go vet ./...

format:
	go fmt ./...

docker-build:
	docker build -f deploy/Dockerfile.gateway -t omnitoken-gateway:local .
	docker build -f deploy/Dockerfile.admin -t omnitoken-admin:local .
	docker build -f deploy/Dockerfile.migrate -t omnitoken-migrate:local .

clean:
	go clean ./...
