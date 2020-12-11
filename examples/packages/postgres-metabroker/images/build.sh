#!/bin/bash

set -o errexit -o nounset -o pipefail

: "${IMAGE_TAG:=postgres-metabroker-credential}"

cd "$(dirname "${0}")"

docker buildx build \
    --tag "${IMAGE_TAG}" \
    --file "Dockerfile.credential" .
