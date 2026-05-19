.PHONY: help up down logs test test-race lint vet format docker-build clean

COMPOSE := docker compose --env-file .env -f deploy/docker-compose.yml
GO_RACE_IMAGE := golang:1.25

help:
	@echo "OmniToken commands:"
	@echo "  make up            Start local Postgres, Redis, NATS, gateway, and admin"
	@echo "  make down          Stop local services"
	@echo "  make logs          Follow compose logs"
	@echo "  make test          Run Go tests"
	@echo "  make test-race     Run Go race tests in Docker"
	@echo "  make lint          Run Go vet"
	@echo "  make format        Format Go code"
	@echo "  make docker-build  Build gateway, admin, and migrate container images"

up:
	docker build -f deploy/Dockerfile.gateway -t omnitoken-gateway:local .
	docker build -f deploy/Dockerfile.admin -t omnitoken-admin:local .
	docker build -f deploy/Dockerfile.migrate -t omnitoken-migrate:local .
	$(COMPOSE) up -d --no-build

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f

test:
	go test ./...

test-race:
	docker run --rm -v "$(CURDIR):/workspace" -v omnitoken-go-mod:/go/pkg/mod -v omnitoken-go-build:/root/.cache/go-build -w /workspace $(GO_RACE_IMAGE) go test -race ./...

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
