#!/usr/bin/env bash
set -e

cd "$(dirname "$0")/.."

CGO_ENABLED=1 go build -ldflags="-s -w" -o media-rpc .
echo "Built: media-rpc"
