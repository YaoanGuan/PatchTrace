#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$ROOT_DIR/webui"
RELEASE_DIR="$ROOT_DIR/release"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) GOARCH="amd64" ;;
  aarch64|arm64) GOARCH="arm64" ;;
  *) echo "不支持的架构: $ARCH"; exit 1 ;;
esac

echo "=========================================="
echo "  AIxVuln Web-only 构建脚本"
echo "  系统: ${OS}/${GOARCH}"
echo "=========================================="

mkdir -p "$RELEASE_DIR"
rm -rf "$RELEASE_DIR/resource"
if [ -d "$ROOT_DIR/resource/tools" ]; then
  mkdir -p "$RELEASE_DIR/resource"
  cp -R "$ROOT_DIR/resource/tools" "$RELEASE_DIR/resource/tools"
fi
if [ -d "$ROOT_DIR/resource/docs" ]; then
  mkdir -p "$RELEASE_DIR/resource"
  cp -R "$ROOT_DIR/resource/docs" "$RELEASE_DIR/resource/docs"
fi
if [ -d "$ROOT_DIR/resource/pocs" ]; then
  mkdir -p "$RELEASE_DIR/resource"
  cp -R "$ROOT_DIR/resource/pocs" "$RELEASE_DIR/resource/pocs"
fi

echo ""
echo "[1/2] 构建前端..."
(
  cd "$FRONTEND_DIR"
  npm install --silent
  npm run build
)
echo "✅ Frontend: $FRONTEND_DIR/dist"

echo ""
echo "[2/2] 构建 Web-only 版本..."
WEB_DST="$RELEASE_DIR/AIxVulnWeb-${OS}-${GOARCH}"
(
  cd "$ROOT_DIR"
  CGO_ENABLED=0 go build -o "$WEB_DST" .
)
chmod +x "$WEB_DST" || true
echo "✅ Web: $WEB_DST"

echo ""
echo "=========================================="
echo "  构建完成！产物目录: $RELEASE_DIR"
ls -lh "$RELEASE_DIR"/ 2>/dev/null || true
echo "=========================================="
