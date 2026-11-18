#!/usr/bin/env bash
# Verify that the hand-written Package Management section is restored after
# a generate-crdocs-style overwrite of docs/programs.md.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
printf '# API Reference\n\n' > "$tmp"
cat "$root/docs/programs-package-management.md" >> "$tmp"
grep -q '^## Package Management$' "$tmp"
grep -q 'The `packages` field allows you' "$tmp"
# And the in-tree generated file must currently include it.
grep -q '^## Package Management$' "$root/docs/programs.md"
grep -q 'The `packages` field allows you' "$root/docs/programs.md"
echo "ok"
