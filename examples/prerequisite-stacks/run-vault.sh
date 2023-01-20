# This runs Vault in the local cluster, according to the instructions for "dev mode" given in
# https://developer.hashicorp.com/vault/docs/platform/k8s/helm/run.

set -eux
set -o pipefail

helm repo add hashicorp https://helm.releases.hashicorp.com
helm repo update

helm upgrade -i vault hashicorp/vault --set "server.dev.enabled=true" --wait

# In the dev mode, the root token is `root`. (You can check this in the pod logs: `kubectl logs
# vault-0`).
