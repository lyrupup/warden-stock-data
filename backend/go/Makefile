.PHONY: test build run backfill tidy infra-up

# 统一环境变量在 backend/.env（go 与 python 共用），相对本目录为上一级。
ENV_FILE := ../.env

test:
	go test ./... -count=1

build:
	go build -o bin/warden-server ./cmd/server

run:
	@test -f $(ENV_FILE) || (echo "请先执行: cp ../.env.example ../.env" && exit 1)
	@set -a && . $(ENV_FILE) && set +a && go run ./cmd/server

backfill:
	@test -f $(ENV_FILE) || (echo "请先执行: cp ../.env.example ../.env" && exit 1)
	@set -a && . $(ENV_FILE) && set +a && go run ./cmd/backfill -years=5

tidy:
	go mod tidy

# 本地开发：仅拉起基建（postgres + redis）；go/python 用各自 make run 本机直跑。
infra-up:
	@test -f $(ENV_FILE) || (echo "请先执行: cp ../.env.example ../.env" && exit 1)
	cd ../deploy && docker compose --env-file ../.env up -d postgres redis
