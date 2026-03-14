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
		WithPublisher("lokkju").
		WithRepository("https://github.com/lokkju/pulumi-improvmx").
		WithLogoURL("https://raw.githubusercontent.com/lokkju/pulumi-improvmx/main/docs/improvmx-logo.png").
		WithPluginDownloadURL("github://api.github.com/lokkju/pulumi-improvmx").
		WithKeywords("kind/native", "category/utility").
		WithConfig(infer.Config(&ProviderConfig{})).
		WithResources(
			infer.Resource(Domain{}),
			infer.Resource(EmailAlias{}),
			infer.Resource(SmtpCredential{}),
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
	ApiToken     string `pulumi:"apiToken,optional" provider:"secret"`
	AutoCheckDNS *bool  `pulumi:"autoCheckDns,optional"`
}

func (c *ProviderConfig) Annotate(a infer.Annotator) {
	a.Describe(&c.ApiToken, "The ImprovMX API token. Can also be set via IMPROVMX_API_TOKEN env var.")
	a.Describe(&c.AutoCheckDNS, "Automatically trigger DNS validation after domain create/update/read to activate forwarding. Defaults to true.")
}
