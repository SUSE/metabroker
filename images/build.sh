#!/bin/bash

set -o errexit -o nounset -o pipefail

: "${IMAGE_TAG:=metabroker-provisioning}"

cd "$(dirname "${0}")"

docker buildx build \
    --tag "${IMAGE_TAG}" \
    --file "Dockerfile.provisioning" .
