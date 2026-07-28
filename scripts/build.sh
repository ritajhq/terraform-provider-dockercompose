#!/usr/bin/env bash
# Builds the terraform-provider-dockercompose binary using Docker only (no local Go needed).
# Builds the image, copies the binary out of a throwaway container, then cleans up.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

IMAGE_TAG="terraform-provider-dockercompose-build"
OUT_DIR="./dist"
OUT_BIN="terraform-provider-dockercompose"

docker build -f Dockerfile.build -t "$IMAGE_TAG" .

CONTAINER_ID=$(docker create "$IMAGE_TAG")
trap 'docker rm -f "$CONTAINER_ID" >/dev/null' EXIT

mkdir -p "$OUT_DIR"
docker cp "$CONTAINER_ID:/out/$OUT_BIN" "$OUT_DIR/$OUT_BIN"

echo "Built $OUT_DIR/$OUT_BIN"
