.PHONY: test build run backfill tidy infra-up

ENV_FILE := .env

test:
	go test ./... -count=1

build:
	go build -o bin/warden-server ./cmd/server

run:
	@test -f $(ENV_FILE) || (echo "请先执行: cp .env.example .env" && exit 1)
	@set -a && . ./$(ENV_FILE) && set +a && go run ./cmd/server

backfill:
	@test -f $(ENV_FILE) || (echo "请先执行: cp .env.example .env" && exit 1)
	@set -a && . ./$(ENV_FILE) && set +a && go run ./cmd/backfill -years=5

tidy:
	go mod tidy

infra-up:
	@test -f $(ENV_FILE) || (echo "请先执行: cp .env.example .env" && exit 1)
	cd deploy && docker compose --env-file ../.env up -d postgres redis
