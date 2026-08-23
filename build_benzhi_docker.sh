#!/bin/sh
set -eu

export DOCKER_DEFAULT_PLATFORM="${DOCKER_DEFAULT_PLATFORM:-linux/amd64}"
docker build -f benzhi.Dockerfile -t "${IMAGE_NAME:-forestpulse:local}" .

