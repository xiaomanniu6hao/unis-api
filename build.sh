#!/usr/bin/env bash
# build.sh — 一键构建 unis-api 本地镜像（含你的二开改动）
#
# 用法:
#   bash build.sh              # 构建镜像 unis-api:local
#   bash build.sh mytag         # 构建镜像 unis-api:mytag
#
# 构建完成后，docker-compose.yml 会用本地镜像而不是拉官方 calciumion/new-api:latest。
# 启动: docker-compose up -d
#
# 说明:
#   - 使用项目根目录的多阶段 Dockerfile（前端 bun 打包 → Go 编译 → 运行时）
#   - 不依赖宿主机装 Go / bun，全部在容器内完成
#   - 构建产物镜像名默认 unis-api，标签 local
# ---------------------------------------------------------------------------

set -euo pipefail

cd "$(dirname "$0")"

TAG="${1:-local}"
IMAGE="unis-api:${TAG}"

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
say()  { echo -e "${GREEN}[build]${NC} $*"; }
warn() { echo -e "${YELLOW}[build]${NC} $*"; }
err()  { echo -e "${RED}[build]${NC} $*" >&2; }

if ! command -v docker >/dev/null 2>&1; then
  err "找不到 docker。请先安装 Docker。"
  exit 1
fi

say "开始构建镜像 ${IMAGE} ..."
say "（首次构建会拉取 bun / golang / debian 基础镜像，耗时较长；后续有缓存会快很多）"

docker build -t "${IMAGE}" .

say "构建完成: ${IMAGE}"
say "验证镜像:"
docker images "${IMAGE}"

say ""
say "下一步: 修改 docker-compose.yml 的 new-api.image 为 ${IMAGE}，然后:"
say "  docker-compose up -d"
say "查看日志: docker logs -f new-api"
