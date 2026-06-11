#!/usr/bin/env bash
# 线上 Docker 一键部署：git pull 后在服务器执行
#   cd backend/deploy && ./deploy.sh
# 一次性部署：postgres + redis + quant(python) + backend(go)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${BACKEND_DIR}/.env"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "[deploy] 错误: 未找到 ${ENV_FILE}"
  echo "[deploy] 请先执行: cp ${BACKEND_DIR}/.env.example ${ENV_FILE} 并填写生产配置"
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "${ENV_FILE}"
set +a

APP_PORT="${APP_PORT:-8080}"

echo "[deploy] 环境文件: ${ENV_FILE}"
echo "[deploy] APP_ENV=${APP_ENV:-unknown}  MARKET_PROVIDER=${MARKET_PROVIDER:-unknown}"

cd "${SCRIPT_DIR}"

echo "[deploy] 构建 backend(go) / quant(python) 镜像..."
docker compose --env-file "${ENV_FILE}" build backend quant

echo "[deploy] 启动全部服务 (postgres / redis / quant / backend)..."
docker compose --env-file "${ENV_FILE}" up -d

echo "[deploy] 等待 backend 健康检查 (http://localhost:${APP_PORT}/health)..."
for i in $(seq 1 60); do
  if curl -sf "http://localhost:${APP_PORT}/health" >/dev/null 2>&1; then
    echo "[deploy] 部署成功"
    docker compose --env-file "${ENV_FILE}" ps
    exit 0
  fi
  sleep 2
done

echo "[deploy] 错误: 健康检查超时，请查看日志: docker compose --env-file ${ENV_FILE} logs backend"
exit 1
