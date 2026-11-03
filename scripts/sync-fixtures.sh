#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

source_dir="${WEAVE_REPO:-../weave}/contract/v1/golden/extension-ipc"
if [ ! -d "$source_dir" ]; then
  echo "missing source fixtures: $source_dir" >&2
  echo "set WEAVE_REPO to a weave checkout" >&2
  exit 1
fi

cp "$source_dir"/*.json testdata/extension-ipc/
echo "synced extension IPC fixtures from $source_dir"
