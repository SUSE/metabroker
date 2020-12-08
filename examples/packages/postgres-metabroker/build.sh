#!/bin/bash

set -o errexit -o nounset -o pipefail

: "${IMAGE_TAG:=postgres-metabroker-credential}"

git_root=$(git rev-parse --show-toplevel)

docker buildx build \
    --tag "${IMAGE_TAG}" \
    --file "${git_root}/examples/packages/postgres-metabroker/images/Dockerfile.credential" \
    "${git_root}/examples/packages/postgres-metabroker/images/"
