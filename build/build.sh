#!/bin/bash
set -e

# Update the CRD type.
echo "Updating the generated CRD API types..."
go run sigs.k8s.io/controller-tools/cmd/controller-gen crd paths=./pkg/apis output:crd:dir=./deploy/crds

# Update the CRD k8s manifests.
echo "Updating the CRD manifests..."
go run sigs.k8s.io/controller-tools/cmd/controller-gen schemapatch:manifests=./deploy/crds output:dir=./deploy/crds paths=./pkg/apis/...

# Build the operator container image.
echo "Building the operator container image..."
IMAGE_NAME="pulumi/pulumi-kubernetes-operator"
IMAGE_TAG="v0.1.0"
operator-sdk build $IMAGE_NAME:$IMAGE_TAG
docker push $IMAGE_NAME:$IMAGE_TAG
