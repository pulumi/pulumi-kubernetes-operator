#!/bin/sh
set -e

echo "Updating the CRD k8s deepcopy code..."
operator-sdk generate k8s
