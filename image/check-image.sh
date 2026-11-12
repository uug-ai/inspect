#!/usr/bin/env bash
set -euo pipefail
image="${1:?usage: image/check-image.sh IMAGE ARCHITECTURE OUTPUT_DIR}"
architecture="${2:?missing architecture}"
output="${3:?missing output directory}"
mkdir -p "$output"
docker image inspect "$image" > "$output/image-inspect.json"
jq -e --arg architecture "$architecture" '
  .[0] | .Architecture == $architecture and
  .Os == "linux" and .Config.User == "65532:65532" and
  .Config.WorkingDir == "/app" and .Config.Entrypoint == ["/app/main"] and
  ((.Config.ExposedPorts // {}) | length) == 0
' "$output/image-inspect.json"
docker image save "$image" -o "$output/image.tar"
: > "$output/rootfs-permissions.txt"
: > "$output/rootfs-files.txt"
index=0
while read -r layer; do
  index=$((index + 1))
  archive="$output/layer-$index.tar"
  tar -xOf "$output/image.tar" "$layer" > "$archive"
  tar --numeric-owner -tvf "$archive" >> "$output/rootfs-permissions.txt"
  tar -tf "$archive" >> "$output/rootfs-files.txt"
  binary=$(tar -tf "$archive" | grep -E '^(\./)?app/main$' || true)
  if [[ -n "$binary" ]]; then
    tar -xOf "$archive" "$binary" > "$output/main"
  fi
done < <(tar -xOf "$output/image.tar" manifest.json | jq -r '.[0].Layers[]')
while read -r mode owner _size _date _time path; do
  path="${path#./}"
  test "$owner" = 0/0
  case "$mode" in
    drwxr-xr-x|dr-xr-xr-x) ;;
    -r-xr-xr-x) test "$path" = app/main ;;
    -r--r--r--) test "$path" = etc/ssl/certs/ca-certificates.crt ;;
    *) echo "unexpected runtime file or permission: $path $mode" >&2; exit 1 ;;
  esac
done < "$output/rootfs-permissions.txt"
grep -Eq '^(\./)?etc/ssl/certs/ca-certificates.crt$' "$output/rootfs-files.txt"
for directory in etc etc/ssl etc/ssl/certs; do
  awk -v expected="$directory" '
    $1 ~ /^d/ {
      path = $6
      sub(/^\.\//, "", path)
      sub(/\/$/, "", path)
      if (path == expected) {
        found = 1
        if ($1 != "drwxr-xr-x" || $2 != "0/0") exit 1
      }
    }
    END { if (!found) exit 1 }
  ' "$output/rootfs-permissions.txt"
done
test -s "$output/main"
if readelf -l "$output/main" | grep -q INTERP; then
  echo "runtime binary is dynamically linked" >&2
  exit 1
fi
readelf -SW "$output/main" > "$output/section-headers.txt"
grep -q '\.symtab' "$output/section-headers.txt"
go version -m "$output/main" > "$output/build-info.txt"
