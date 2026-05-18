#!/usr/bin/env bash
# scripts/sync-backend-docs.sh
# Mirror the dun backend's public docs into docs/backend/.
# Source: https://github.com/fguillen/dun
#
# Usage:
#   ./scripts/sync-backend-docs.sh              # pulls from main
#   DUN_REF=v1.0.0 ./scripts/sync-backend-docs.sh   # pin to tag/branch/sha

set -euo pipefail

REPO="fguillen/dun"
REF="${DUN_REF:-main}"
BASE="https://raw.githubusercontent.com/${REPO}/${REF}"
DEST="docs/backend"

# source-path-in-repo -> destination-filename-in-docs/backend
FILES=(
  "docs/openapi.yaml|openapi.yaml"
  "docs/tutorial.md|tutorial.md"
  "docs/dun Game Design Document.v3.md|game-design.md"
  "PRODUCT.md|product.md"
  "docs/architecture/api-endpoints.md|api-endpoints.md"
  "README.md|backend-readme.md"
)

mkdir -p "$DEST"

# urlencode that handles spaces (the only non-ASCII char we hit is ' ').
urlencode() { printf '%s' "$1" | sed -e 's/ /%20/g'; }

echo "Syncing backend docs from ${REPO}@${REF}..."
for entry in "${FILES[@]}"; do
  src="${entry%%|*}"
  dst="${entry##*|}"
  url="${BASE}/$(urlencode "$src")"
  out="${DEST}/${dst}"
  echo "  ${src} -> ${out}"
  curl --fail --silent --show-error --location "$url" --output "$out"
done

# Write a small manifest so future-you can tell what ref this snapshot came from.
cat > "${DEST}/SOURCE.md" <<EOF
# Backend docs snapshot

Mirrored from **https://github.com/${REPO}** at ref \`${REF}\`
on $(date -u +"%Y-%m-%d %H:%M:%SZ").

Refresh with \`./scripts/sync-backend-docs.sh\` (set \`DUN_REF\` to pin).

Do not edit files in this directory by hand — they will be overwritten on
the next sync. Treat them as read-only reference material.
EOF

echo "Done. Snapshot manifest at ${DEST}/SOURCE.md."
