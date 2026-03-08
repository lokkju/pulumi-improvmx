package provider

import (
	"fmt"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

var Version string

const Name string = "improvmx"

func Provider() p.Provider {
	p, err := infer.NewProviderBuilder().
		WithDisplayName("ImprovMX").
		WithDescription("Manage ImprovMX email forwarding resources.").
		WithHomepage("https://improvmx.com").
		WithNamespace("lokkju").
		WithConfig(infer.Config(&ProviderConfig{})).
		WithResources(
			infer.Resource(Domain{}),
		).
		WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{
			"provider": "index",
		}).
		Build()
	if err != nil {
		panic(fmt.Errorf("unable to build provider: %w", err))
	}
	return p
}

type ProviderConfig struct {
	ApiToken string `pulumi:"apiToken,optional" provider:"secret"`
}

func (c *ProviderConfig) Annotate(a infer.Annotator) {
	a.Describe(&c.ApiToken, "The ImprovMX API token. Can also be set via IMPROVMX_API_TOKEN env var.")
}
