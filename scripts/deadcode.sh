#!/bin/sh
# golang.org/x/tools/cmd/deadcode reports functions that cannot be reached
# from program entrypoints (main). The tool exits 0 even when it finds some,
# so this script exits 1 when that report is non-empty.
set -eu

cd "$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"

mod_go=$(awk '/^go / { print $2; exit }' go.mod)
# golang.org/x/tools selects a Go 1.26 toolchain, which cannot load this
# module. Run deadcode with the Go version declared in go.mod.
export GOTOOLCHAIN="go${mod_go}"

out=$(mktemp)
trap 'rm -f "$out"' EXIT

set +e
go run golang.org/x/tools/cmd/deadcode@v0.51.0 ./... >"$out"
status=$?
set -e

cat "$out"
if [ "$status" -ne 0 ]; then
	exit "$status"
fi
if [ -s "$out" ]; then
	echo "deadcode: unreachable functions found" >&2
	exit 1
fi
