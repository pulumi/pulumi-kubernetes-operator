#!/bin/bash
set -e

# Update the generated resource type.
echo "Updating the generated CRD API types..."
operator-sdk generate k8s

# Update the CRD manifests.
echo "Updating the CRD manifests..."
operator-sdk generate crds

# Build the operator container image.
echo "Building the operator container image..."
IMAGE_NAME="pulumi/pulumi-kubernetes-operator"
IMAGE_TAG="v0.1.0"
operator-sdk build $IMAGE_NAME:$IMAGE_TAG
# docker push $IMAGE_TAG
