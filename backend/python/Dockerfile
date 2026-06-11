# Python quant 服务镜像（采集 + 指标）。
# 使用 slim（debian/glibc）而非 alpine：baostock/pandas/numpy 在 musl 上易踩坑。
FROM python:3.12-slim

WORKDIR /app

# 仅装运行所需系统依赖（psycopg 二进制轮子自带，无需额外编译）
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app ./app

EXPOSE 8000

# 容器内监听 0.0.0.0（仅 compose 内网可达，不映射宿主端口）
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
