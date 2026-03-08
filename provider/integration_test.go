package provider

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipIfNoLiveAPI(t *testing.T) {
	t.Helper()
	if os.Getenv("IMPROVMX_LIVE_TEST") != "1" {
		t.Skip("Set IMPROVMX_LIVE_TEST=1 and IMPROVMX_API_TOKEN + IMPROVMX_TEST_DOMAIN to run")
	}
}

func liveClient(t *testing.T) (*ImprovMXClient, string) {
	t.Helper()
	token := os.Getenv("IMPROVMX_API_TOKEN")
	domain := os.Getenv("IMPROVMX_TEST_DOMAIN")
	require.NotEmpty(t, token, "IMPROVMX_API_TOKEN required")
	require.NotEmpty(t, domain, "IMPROVMX_TEST_DOMAIN required")
	return NewImprovMXClient(token), domain
}

func TestLiveDomainCRUD(t *testing.T) {
	skipIfNoLiveAPI(t)
	client, testDomain := liveClient(t)

	// Ensure domain exists (create or get)
	_, err := client.CreateDomain(testDomain, "")
	if err != nil {
		// May already exist
		_, err = client.GetDomain(testDomain)
		require.NoError(t, err)
	}

	// Read
	domain, err := client.GetDomain(testDomain)
	require.NoError(t, err)
	assert.Equal(t, testDomain, domain.Domain)

	// List
	domains, err := client.ListDomains()
	require.NoError(t, err)
	found := false
	for _, d := range domains {
		if d.Domain == testDomain {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestLiveAliasCRUD(t *testing.T) {
	skipIfNoLiveAPI(t)
	client, testDomain := liveClient(t)

	// Cleanup first
	client.DeleteAlias(testDomain, "pulumi-test")

	// Create
	alias, err := client.CreateAlias(testDomain, "pulumi-test", "test@example.com")
	require.NoError(t, err)
	assert.Equal(t, "pulumi-test", alias.Alias)

	// Read
	fetched, err := client.GetAlias(testDomain, "pulumi-test")
	require.NoError(t, err)
	assert.Contains(t, fetched.Forward, "test@example.com")

	// Update
	updated, err := client.UpdateAlias(testDomain, "pulumi-test", "updated@example.com")
	require.NoError(t, err)
	assert.Contains(t, updated.Forward, "updated@example.com")

	// Delete
	err = client.DeleteAlias(testDomain, "pulumi-test")
	require.NoError(t, err)
}

func TestLiveWildcardAlias(t *testing.T) {
	skipIfNoLiveAPI(t)
	client, testDomain := liveClient(t)

	client.DeleteAlias(testDomain, "*")

	alias, err := client.CreateAlias(testDomain, "*", "catchall@example.com")
	require.NoError(t, err)
	assert.Equal(t, "*", alias.Alias)

	err = client.DeleteAlias(testDomain, "*")
	require.NoError(t, err)
}

func TestLiveSmtpCredentialCRUD(t *testing.T) {
	skipIfNoLiveAPI(t)
	client, testDomain := liveClient(t)

	// Cleanup
	client.DeleteSmtpCredential(testDomain, "pulumi-test-sender")

	// Create
	cred, err := client.CreateSmtpCredential(testDomain, "pulumi-test-sender", "T3stP@ssw0rd!")
	require.NoError(t, err)
	assert.Equal(t, "pulumi-test-sender", cred.Username)

	// List and find
	creds, err := client.ListSmtpCredentials(testDomain)
	require.NoError(t, err)
	found := false
	for _, c := range creds {
		if c.Username == "pulumi-test-sender" {
			found = true
			break
		}
	}
	assert.True(t, found)

	// Update password
	err = client.UpdateSmtpCredential(testDomain, "pulumi-test-sender", "N3wP@ssw0rd!")
	require.NoError(t, err)

	// Delete
	err = client.DeleteSmtpCredential(testDomain, "pulumi-test-sender")
	require.NoError(t, err)
}
