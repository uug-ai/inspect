#!/usr/bin/env bash
# Hardened image smoke test, run by uug-ai/workflows release-create (profile standalone).
set -euo pipefail
image="${1:?usage: image/smoke-test.sh IMAGE}"
architecture=$(docker image inspect "$image" --format '{{.Architecture}}')
bash image/check-image.sh "$image" "$architecture" "${RUNTIME_EVIDENCE_DIR:-.build/evidence}"
args=(--rm --network none --read-only --cap-drop ALL
  --security-opt no-new-privileges --pids-limit 32 --memory 128m --cpus 1)

# Offline: certificates load and the HTTP handler answers /healthz on loopback.
# Never contacts GitHub (the self-test uses a dry-run, local-store configuration).
timeout 30 docker run "${args[@]}" "$image" --self-test

# Unknown arguments must fail with usage (exit 2), not start the server.
if timeout 30 docker run "${args[@]}" "$image" --invalid-option; then
  echo "invalid arguments must not start the server" >&2
  exit 1
else
  test "$?" -eq 2
fi

# Without a token the server must refuse to start (exit 1) rather than run half-configured.
if timeout 30 docker run "${args[@]}" -e INSPECT_DEFAULT_REPO=uug-ai/inspect "$image"; then
  echo "the server must not start without GITHUB_TOKEN" >&2
  exit 1
else
  test "$?" -eq 1
fi

# Resolve an alias through Docker's DNS on an internal-only network, never a real service.
network="inspect-smoke-${GITHUB_RUN_ID:-local}-${GITHUB_RUN_ATTEMPT:-0}-$$"
docker network create --internal "$network" >/dev/null
trap 'docker network rm "$network" >/dev/null' EXIT
timeout 30 docker run --rm --network "$network" --network-alias inspect-runtime-ci \
  --read-only --cap-drop ALL --security-opt no-new-privileges \
  --pids-limit 32 --memory 128m --cpus 1 \
  "$image" --self-test-dns inspect-runtime-ci
