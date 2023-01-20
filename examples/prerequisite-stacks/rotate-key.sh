vault_temp_token=$(VAULT_TOKEN="$1" vault token create -ttl=10m -format=json | jq -r '.auth.client_token')
kubectl delete secret --ignore-not-found vault-access
kubectl create secret generic vault-access --from-literal=VAULT_TOKEN=$vault_temp_token
