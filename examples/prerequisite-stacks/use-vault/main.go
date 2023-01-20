package main

// Adapted from https://github.com/phillipedwards/sf-vault-repro/blob/master/main.go

import (
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/pulumi/pulumi-vault/sdk/v4/go/vault"
	"github.com/pulumi/pulumi-vault/sdk/v4/go/vault/generic"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		vaultProvider, err := vault.NewProvider(ctx, "vault", &vault.ProviderArgs{
			Token:   pulumi.String(os.Getenv("VAULT_TOKEN")),
			Address: pulumi.String(os.Getenv("VAULT_ADDR")),
		})

		if err != nil {
			return err
		}

		// this will fail if the credentials used for the provider have expired.
		_, err = generic.NewSecret(ctx, "secret-1", &generic.SecretArgs{
			Path: pulumi.String("secret/test-path"),
			DataJson: pulumi.String(`{
				"key": "value"
			}
			`),
		}, pulumi.Provider(vaultProvider))

		return err
	})
}
