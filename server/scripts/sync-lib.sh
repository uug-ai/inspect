#!/usr/bin/env bash
# Build the browser library and copy it where the landing page embeds it, so
# `go run .` serves a working "Try it" demo locally. Release images do this in
# server/Dockerfile.
set -euo pipefail
root="$(cd "$(dirname "$0")/../.." && pwd)"
(cd "$root/client" && { [ -d node_modules ] || npm ci; } && npx vite build)
mkdir -p "$root/server/internal/site/web/lib"
cp "$root/client/dist/inspect.umd.cjs" "$root/server/internal/site/web/lib/inspect.js"
echo "copied client/dist/inspect.umd.cjs -> server/internal/site/web/lib/inspect.js"
