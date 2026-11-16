#!/usr/bin/env bash
#
# AIxVuln 一键交叉编译脚本（Web-only）
#
# 产出目录: dist/
#   dist/AIxVuln-web-<os>-<arch>[.exe]           — WEB 版可执行文件
#   dist/AIxVuln-web-<os>-<arch>[.exe].tar.gz    — WEB 版压缩包
#
# 用法:
#   ./build_all.sh              # 构建全部 WEB 目标
#   ./build_all.sh web          # 同上，显式指定 WEB 模式
#   ./build_all.sh web linux    # 仅构建 WEB 版 Linux 目标
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$SCRIPT_DIR"
FRONTEND_DIR="$PROJECT_DIR/webui"
DIST_DIR="$PROJECT_DIR/dist"

BUILD_MODE="${1:-web}"   # 仅支持 web / all（兼容旧调用）
OS_FILTER="${2:-}"       # 可选: darwin | linux | windows

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
fail()  { echo -e "${RED}[FAIL]${NC}  $*"; }

check_tool() {
    if ! command -v "$1" &>/dev/null; then
        fail "需要 $1，请先安装: $2"
        return 1
    fi
}

check_tool go   "https://go.dev/dl/"
check_tool node "https://nodejs.org/"
check_tool npm  "https://nodejs.org/"

mkdir -p "$DIST_DIR"

info "构建前端..."
(cd "$FRONTEND_DIR" && npm install --silent && npm run build)
ok "前端构建完成"

package_binary() {
    local bin_path="$1"
    if [ ! -f "$bin_path" ]; then
        fail "打包失败，文件不存在: $bin_path"
        return 1
    fi

    local bin_dir bin_name archive
    bin_dir="$(dirname "$bin_path")"
    bin_name="$(basename "$bin_path")"
    archive="${bin_path}.tar.gz"

    rm -f "$archive"

    local stage_dir
    stage_dir="$(mktemp -d "$bin_dir/.pkg.XXXXXX")"
    cp "$bin_path" "$stage_dir/$bin_name"
    if [ -d "$PROJECT_DIR/resource/tools" ]; then
        mkdir -p "$stage_dir/resource"
        cp -R "$PROJECT_DIR/resource/tools" "$stage_dir/resource/tools"
    fi
    if [ -d "$PROJECT_DIR/resource/docs" ]; then
        mkdir -p "$stage_dir/resource"
        cp -R "$PROJECT_DIR/resource/docs" "$stage_dir/resource/docs"
    fi
    if [ -d "$PROJECT_DIR/resource/pocs" ]; then
        mkdir -p "$stage_dir/resource"
        cp -R "$PROJECT_DIR/resource/pocs" "$stage_dir/resource/pocs"
    fi
    (
        cd "$stage_dir"
        if [ -d "resource" ]; then
            tar -czf "$archive" "$bin_name" "resource"
        else
            tar -czf "$archive" "$bin_name"
        fi
    )
    rm -rf "$stage_dir"
    rm -f "$bin_path"
    ok "打包完成 -> $(basename "$archive")"
}

build_web() {
    local os="$1" arch="$2"
    local ext=""
    [ "$os" = "windows" ] && ext=".exe"
    local out="$DIST_DIR/AIxVuln-web-${os}-${arch}${ext}"

    info "WEB  ${os}/${arch} ..."
    (
        cd "$PROJECT_DIR"
        CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
            go build -ldflags="-s -w" -o "$out" .
    )
    ok "WEB  ${os}/${arch} -> $(basename "$out")"
    package_binary "$out"
}

should_build() {
    [ -z "$OS_FILTER" ] || [ "$OS_FILTER" = "$1" ]
}

build_all_web() {
    info "========== WEB 版构建 (CGO_ENABLED=0) =========="
#    should_build darwin  && build_web darwin  amd64
#    should_build darwin  && build_web darwin  arm64
    should_build linux   && build_web linux   amd64
#    should_build linux   && build_web linux   arm64
#    should_build windows && build_web windows amd64
#    should_build windows && build_web windows arm64
}

echo ""
echo "╔══════════════════════════════════════════════╗"
echo "║       AIxVuln 交叉编译构建                   ║"
echo "╠══════════════════════════════════════════════╣"
echo "║  模式: $(printf '%-37s' "web") ║"
echo "║  平台: $(printf '%-37s' "${OS_FILTER:-全部}") ║"
echo "╚══════════════════════════════════════════════╝"
echo ""

case "$BUILD_MODE" in
    web|all)
        build_all_web
        ;;
    *)
        fail "未知模式: $BUILD_MODE (当前仅支持: web)"
        exit 1
        ;;
esac

echo ""
info "========== 构建产出 =========="
ls -lh "$DIST_DIR"/AIxVuln-web-* 2>/dev/null || warn "没有构建产出"
echo ""
ok "构建完成！产出目录: $DIST_DIR/"
