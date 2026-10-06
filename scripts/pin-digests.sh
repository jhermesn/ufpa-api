#!/usr/bin/env bash
set -euo pipefail

dockerfile="${1:-Dockerfile}"

list_unpinned_images() {
  sed -nE 's/^FROM[[:space:]]+(--platform=[^[:space:]]+[[:space:]]+)?([^-@[:space:]][^@[:space:]]*)([[:space:]].*)?$/\2/p' "$dockerfile"
}

resolve_digest() {
  docker buildx imagetools inspect "$1" --format '{{json .Manifest.Digest}}' | tr -d '"'
}

pin_image() {
  local image="$1" digest="$2" escaped pinned
  escaped="$(printf '%s' "$image" | sed 's/[.]/\\./g')"
  pinned="$(sed -E "s#^(FROM[[:space:]]+(--platform=[^[:space:]]+[[:space:]]+)?)${escaped}([[:space:]]|$)#\1${image}@${digest}\3#" "$dockerfile")"
  printf '%s\n' "$pinned" > "$dockerfile"
}

images="$(list_unpinned_images)"
for image in $images; do
  digest="$(resolve_digest "$image")"
  [[ "$digest" == sha256:* ]] || { echo "failed to resolve digest for $image" >&2; exit 1; }
  pin_image "$image" "$digest"
  echo "pinned $image -> $digest"
done
