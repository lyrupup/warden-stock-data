.PHONY: run test install backfill

# 统一环境变量在 backend/.env（go 与 python 共用），相对本目录为上一级。
ENV_FILE := ../.env

install:
	pip install -r requirements.txt

run:
	@test -f $(ENV_FILE) || (echo "请先执行: cp ../.env.example ../.env" && exit 1)
	@set -a && . $(ENV_FILE) && set +a && uvicorn app.main:app --host 127.0.0.1 --port $${QUANT_PORT:-8000} --reload

# 离线批量回补日 K（直接写库）。示例：
#   make backfill ARGS="--codes 600519,000001"
#   make backfill ARGS="--all --skip-done"
backfill:
	@test -f $(ENV_FILE) || (echo "请先执行: cp ../.env.example ../.env" && exit 1)
	@set -a && . $(ENV_FILE) && set +a && python -m app.scripts.backfill $(ARGS)

test:
	pytest -q
