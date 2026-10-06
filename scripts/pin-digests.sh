#!/usr/bin/env bash
set -euo pipefail

dockerfile="${1:-Dockerfile}"

resolve_digest() {
  docker buildx imagetools inspect "$1" --format '{{json .Manifest.Digest}}' | tr -d '"'
}

pin_image() {
  local image="$1" digest="$2" escaped
  escaped="$(printf '%s' "$image" | sed 's/[.]/\\./g')"
  sed -i -E "s#^(FROM[[:space:]]+(--platform=[^[:space:]]+[[:space:]]+)?)${escaped}([[:space:]]|$)#\1${image}@${digest}\3#" "$dockerfile"
}

for image in $(grep -oP '^FROM\s+(--platform=\S+\s+)?\K(?!--)[^@\s]+(?=\s|$)' "$dockerfile"); do
  digest="$(resolve_digest "$image")"
  [[ "$digest" == sha256:* ]] || { echo "failed to resolve digest for $image" >&2; exit 1; }
  pin_image "$image" "$digest"
  echo "pinned $image -> $digest"
done
