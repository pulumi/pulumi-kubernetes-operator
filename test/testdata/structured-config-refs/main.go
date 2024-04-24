package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		myConfig := config.New(ctx, "")

		structuredConfig := StructuredConfig{}
		myConfig.RequireObject("structured", &structuredConfig)

		ctx.Export("nested-config-field", pulumi.String(structuredConfig.Nested.Field))
		return nil
	})
}

type StructuredConfig struct {
	Nested NestedConfig
}

type NestedConfig struct {
	Field string
}
