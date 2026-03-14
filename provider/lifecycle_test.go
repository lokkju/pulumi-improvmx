package provider

import (
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func lifecycleServer(t *testing.T) integration.Server {
	t.Helper()
	server, err := integration.NewServer(
		context.Background(),
		"improvmx",
		semver.MustParse("0.0.1"),
		integration.WithProvider(Provider()),
	)
	if err != nil {
		t.Fatalf("Failed to create lifecycle server: %v", err)
	}
	return server
}

func TestLifecycleDomain_CreateUpdateDelete(t *testing.T) {
	skipIfNoLiveAPI(t)
	client, testDomain := liveClient(t)
	safetyGate(t, client, testDomain)

	email := "lifecycle-test@example.com"

	integration.LifeCycleTest{
		Resource: "improvmx:index:Domain",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"domain": property.New(testDomain),
			}),
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"domain":            property.New(testDomain),
				"notificationEmail": property.New(email),
			}),
		}},
	}.Run(t, lifecycleServer(t))
}

func TestLifecycleDomain_CreateTwice_Idempotent(t *testing.T) {
	skipIfNoLiveAPI(t)
	client, testDomain := liveClient(t)
	safetyGate(t, client, testDomain)

	server := lifecycleServer(t)

	integration.LifeCycleTest{
		Resource: "improvmx:index:Domain",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"domain": property.New(testDomain),
			}),
		},
	}.Run(t, server)

	integration.LifeCycleTest{
		Resource: "improvmx:index:Domain",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"domain": property.New(testDomain),
			}),
		},
	}.Run(t, server)
}

func TestLifecycleAlias_CreateUpdateDelete(t *testing.T) {
	skipIfNoLiveAPI(t)
	client, testDomain := liveClient(t)
	safetyGate(t, client, testDomain)

	client.CreateDomain(testDomain, "")
	t.Cleanup(func() { client.DeleteDomain(testDomain) })

	integration.LifeCycleTest{
		Resource: "improvmx:index:EmailAlias",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"domain":  property.New(testDomain),
				"alias":   property.New("lifecycle-test"),
				"forward": property.New("create@example.com"),
			}),
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"domain":  property.New(testDomain),
				"alias":   property.New("lifecycle-test"),
				"forward": property.New("updated@example.com"),
			}),
		}},
	}.Run(t, lifecycleServer(t))
}

func TestLifecycleAlias_ReplaceOnAliasChange(t *testing.T) {
	skipIfNoLiveAPI(t)
	client, testDomain := liveClient(t)
	safetyGate(t, client, testDomain)

	client.CreateDomain(testDomain, "")
	t.Cleanup(func() { client.DeleteDomain(testDomain) })

	integration.LifeCycleTest{
		Resource: "improvmx:index:EmailAlias",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"domain":  property.New(testDomain),
				"alias":   property.New("replace-test-old"),
				"forward": property.New("user@example.com"),
			}),
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"domain":  property.New(testDomain),
				"alias":   property.New("replace-test-new"),
				"forward": property.New("user@example.com"),
			}),
		}},
	}.Run(t, lifecycleServer(t))
}

func TestLifecycleCredential_CreateUpdateDelete(t *testing.T) {
	skipIfNoLiveAPI(t)
	client, testDomain := liveClient(t)
	safetyGate(t, client, testDomain)

	client.CreateDomain(testDomain, "")
	t.Cleanup(func() { client.DeleteDomain(testDomain) })

	integration.LifeCycleTest{
		Resource: "improvmx:index:SmtpCredential",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"domain":   property.New(testDomain),
				"username": property.New("lifecycle-sender"),
				"password": property.New("T3stP@ss!").WithSecret(true),
			}),
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"domain":   property.New(testDomain),
				"username": property.New("lifecycle-sender"),
				"password": property.New("N3wP@ss!").WithSecret(true),
			}),
		}},
	}.Run(t, lifecycleServer(t))
}

func TestLifecycleCredential_ReplaceOnUsernameChange(t *testing.T) {
	skipIfNoLiveAPI(t)
	client, testDomain := liveClient(t)
	safetyGate(t, client, testDomain)

	client.CreateDomain(testDomain, "")
	t.Cleanup(func() { client.DeleteDomain(testDomain) })

	integration.LifeCycleTest{
		Resource: "improvmx:index:SmtpCredential",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"domain":   property.New(testDomain),
				"username": property.New("old-sender"),
				"password": property.New("T3stP@ss!").WithSecret(true),
			}),
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"domain":   property.New(testDomain),
				"username": property.New("new-sender"),
				"password": property.New("T3stP@ss!").WithSecret(true),
			}),
		}},
	}.Run(t, lifecycleServer(t))
}
