package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/anthropics-contrib/terraform-provider-anthropic-managed-agents/internal/provider"
)

func main() {
	err := providerserver.Serve(context.Background(), provider.New, providerserver.ServeOpts{
		Address: "registry.terraform.io/anthropics-contrib/anthropic",
	})
	if err != nil {
		log.Fatal(err)
	}
}
