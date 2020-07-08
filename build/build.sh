#!/bin/bash
set -e

# Update the CRD type.
echo "Updating the generated CRD API types..."
operator-sdk generate crds

# Update the CRD k8s manifests.
echo "Updating the CRD manifests..."
operator-sdk generate k8s

# Build the operator container image.
echo "Building the operator container image..."
IMAGE_NAME="pulumi/pulumi-kubernetes-operator"
IMAGE_TAG="v0.1.0"
operator-sdk build $IMAGE_NAME:$IMAGE_TAG
docker push $IMAGE_NAME:$IMAGE_TAG
