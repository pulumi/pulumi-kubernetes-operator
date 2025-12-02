#!/bin/bash
# cleanup-orphaned-updates.sh
#
# Cleans up orphaned Update objects that have:
# - A deletionTimestamp set (marked for deletion)
# - The finalizer.stack.pulumi.com finalizer still present (blocking deletion)
#
# This is a temporary workaround for issue #1060 where a race condition
# can leave Update objects stuck with finalizers that were never removed.

set -euo pipefail

FINALIZER="finalizer.stack.pulumi.com"
DRY_RUN="${DRY_RUN:-true}"

echo "Scanning for orphaned Update objects..."
echo "DRY_RUN=$DRY_RUN (set DRY_RUN=false to actually remove finalizers)"
echo ""

# Get all Updates across all namespaces that have deletionTimestamp set and our finalizer
ORPHANED_UPDATES=$(kubectl get updates.auto.pulumi.com --all-namespaces -o json | \
  jq -r '.items[] | select(.metadata.deletionTimestamp != null) | select(.metadata.finalizers[]? == "'"$FINALIZER"'") | "\(.metadata.namespace)/\(.metadata.name)"')

if [ -z "$ORPHANED_UPDATES" ]; then
  echo "No orphaned Update objects found."
  exit 0
fi

echo "Found orphaned Update objects:"
echo "$ORPHANED_UPDATES"
echo ""

for UPDATE in $ORPHANED_UPDATES; do
  NAMESPACE=$(echo "$UPDATE" | cut -d'/' -f1)
  NAME=$(echo "$UPDATE" | cut -d'/' -f2)

  echo "Processing: $NAMESPACE/$NAME"

  if [ "$DRY_RUN" = "true" ]; then
    echo "  [DRY RUN] Would remove finalizers"
  else
    echo "  Removing finalizers..."
    kubectl patch update.auto.pulumi.com "$NAME" -n "$NAMESPACE" --type=merge \
      -p '{"metadata":{"finalizers":null}}'
    echo "  Done."
  fi
done

echo ""
if [ "$DRY_RUN" = "true" ]; then
  echo "Dry run complete. Run with DRY_RUN=false to actually clean up."
else
  echo "Cleanup complete."
fi
