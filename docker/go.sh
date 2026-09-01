#!/bin/sh
# 在容器內跑 Go 工具鏈。模組來源是主機下載快取的唯讀 file:// proxy，
# 模組解壓與建置快取寫在 scratchpad，不動主機的 ~/go/pkg/mod。
#
#   docker/go.sh test ./...
#   docker/go.sh build ./cmd/acan-headless
set -eu

REPO=$(cd "$(dirname "$0")/.." && pwd)
IMAGE=${ACAN_GO_IMAGE:-superacan-ebitengine:go1.26.7-v1}
HOST_CACHE=${ACAN_HOST_MODCACHE:-$HOME/go/pkg/mod}
WORK=${ACAN_GO_WORK:-${TMPDIR:-/tmp}/superacan-go}

mkdir -p "$WORK/mod" "$WORK/build"

exec timeout "${ACAN_GO_TIMEOUT:-900}" docker run --rm --network none \
    --memory "${ACAN_GO_MEMORY:-6g}" --cpus "${ACAN_GO_CPUS:-4}" --pids-limit 512 \
    --log-opt max-size=10m --log-opt max-file=3 \
    -u "$(id -u):$(id -g)" \
    -v "$REPO:/src" \
    -v "$HOST_CACHE:/hostmod:ro" \
    -v "$WORK:/gowork" \
    ${ACAN_MEDIA_DIR:+-v "$ACAN_MEDIA_DIR:/media:ro"} \
    ${ACAN_BIOS_DIR:+-v "$ACAN_BIOS_DIR:/bios:ro"} \
    -e HOME=/gowork \
    -e GOMODCACHE=/gowork/mod \
    -e GOCACHE=/gowork/build \
    -e GOFLAGS=-mod=mod \
    -e GOPROXY=file:///hostmod/cache/download \
    -e GOSUMDB=off \
    -e GOTOOLCHAIN=local \
    ${ACAN_GOOS:+-e "GOOS=$ACAN_GOOS"} \
    ${ACAN_GOARCH:+-e "GOARCH=$ACAN_GOARCH"} \
    ${ACAN_CGO:+-e "CGO_ENABLED=$ACAN_CGO"} \
    -e "ACAN_UI_DUMP=${ACAN_UI_DUMP:-}" \
    -w /src "$IMAGE" go "$@"
