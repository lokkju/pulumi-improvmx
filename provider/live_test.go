package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipIfNoLiveAPI(t *testing.T) {
	t.Helper()
	if os.Getenv("IMPROVMX_LIVE_TEST") != "1" {
		t.Skip("Set IMPROVMX_LIVE_TEST=1 to run live tests")
	}
}

func liveClient(t *testing.T) (*ImprovMXClient, string) {
	t.Helper()
	token := os.Getenv("IMPROVMX_API_TOKEN")
	domain := os.Getenv("IMPROVMX_INTEGRATION_TEST_DOMAIN")
	require.NotEmpty(t, token, "IMPROVMX_API_TOKEN must be set")
	require.NotEmpty(t, domain, "IMPROVMX_INTEGRATION_TEST_DOMAIN must be set")
	return NewImprovMXClient(token), domain
}

func safetyGate(t *testing.T, client *ImprovMXClient, domain string) {
	t.Helper()

	fmt.Printf("\n")
	fmt.Printf("========================================================================\n")
	fmt.Printf("WARNING: Live tests will CREATE and DELETE resources on domain %s.\n", domain)
	fmt.Printf("         The domain itself WILL be destroyed.\n")
	fmt.Printf("========================================================================\n")
	fmt.Printf("\n")

	d, err := client.GetDomain(domain)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.IsNotFound() {
			return
		}
		t.Fatalf("Failed to check domain %s: %v", domain, err)
	}
	_ = d

	aliases, err := client.ListAliases(domain)
	if err != nil {
		t.Fatalf("Failed to list aliases for %s: %v", domain, err)
	}
	if len(aliases) > 0 {
		t.Fatalf("Refusing to run: domain %s has %d existing aliases. Use a clean test domain.", domain, len(aliases))
	}

	creds, err := client.ListSmtpCredentials(domain)
	if err != nil {
		t.Fatalf("Failed to list credentials for %s: %v", domain, err)
	}
	if len(creds) > 0 {
		t.Fatalf("Refusing to run: domain %s has %d existing SMTP credentials. Use a clean test domain.", domain, len(creds))
	}

	_ = client.DeleteDomain(domain)
}

func TestLive(t *testing.T) {
	skipIfNoLiveAPI(t)
	client, testDomain := liveClient(t)
	safetyGate(t, client, testDomain)

	t.Run("Domain", func(t *testing.T) {
		t.Run("Create", func(t *testing.T) {
			domain, err := client.CreateDomain(testDomain, "")
			require.NoError(t, err)
			assert.Equal(t, testDomain, domain.Domain)
		})
		t.Run("Read", func(t *testing.T) {
			domain, err := client.GetDomain(testDomain)
			require.NoError(t, err)
			assert.Equal(t, testDomain, domain.Domain)
		})
		t.Run("Update_NotificationEmail", func(t *testing.T) {
			domain, err := client.UpdateDomain(testDomain, map[string]string{
				"notification_email": "test-notify@example.com",
			})
			require.NoError(t, err)
			assert.Equal(t, "test-notify@example.com", domain.NotificationEmail)
		})
		t.Run("Read_AfterUpdate", func(t *testing.T) {
			domain, err := client.GetDomain(testDomain)
			require.NoError(t, err)
			assert.Equal(t, "test-notify@example.com", domain.NotificationEmail)
		})
		t.Run("CreateAgain_Adopt", func(t *testing.T) {
			_, err := client.CreateDomain(testDomain, "")
			require.Error(t, err)
			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			assert.True(t, apiErr.IsAlreadyExists())
			domain, err := client.GetDomain(testDomain)
			require.NoError(t, err)
			assert.Equal(t, testDomain, domain.Domain)
		})
		t.Run("Delete", func(t *testing.T) {
			err := client.DeleteDomain(testDomain)
			require.NoError(t, err)
		})
		t.Run("Delete_AlreadyGone", func(t *testing.T) {
			err := client.DeleteDomain(testDomain)
			require.Error(t, err)
			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			assert.True(t, apiErr.IsNotFound())
		})
		t.Run("Read_AfterDelete", func(t *testing.T) {
			_, err := client.GetDomain(testDomain)
			require.Error(t, err)
			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			assert.True(t, apiErr.IsNotFound())
		})
		t.Run("Recreate", func(t *testing.T) {
			domain, err := client.CreateDomain(testDomain, "")
			require.NoError(t, err)
			assert.Equal(t, testDomain, domain.Domain)
		})
		t.Run("Delete_Final", func(t *testing.T) {
			err := client.DeleteDomain(testDomain)
			require.NoError(t, err)
		})
	})

	t.Run("EmailAlias", func(t *testing.T) {
		t.Run("SetupDomain", func(t *testing.T) {
			_, err := client.CreateDomain(testDomain, "")
			require.NoError(t, err)
		})
		t.Run("Create", func(t *testing.T) {
			alias, err := client.CreateAlias(testDomain, "pulumi-test", "test@example.com")
			require.NoError(t, err)
			assert.Equal(t, "pulumi-test", alias.Alias)
		})
		t.Run("Read", func(t *testing.T) {
			alias, err := client.GetAlias(testDomain, "pulumi-test")
			require.NoError(t, err)
			assert.Contains(t, alias.Forward, "test@example.com")
		})
		t.Run("Update_Forward", func(t *testing.T) {
			alias, err := client.UpdateAlias(testDomain, "pulumi-test", "updated@example.com")
			require.NoError(t, err)
			assert.Contains(t, alias.Forward, "updated@example.com")
		})
		t.Run("Read_AfterUpdate", func(t *testing.T) {
			alias, err := client.GetAlias(testDomain, "pulumi-test")
			require.NoError(t, err)
			assert.Contains(t, alias.Forward, "updated@example.com")
		})
		t.Run("CreateAgain_Adopt", func(t *testing.T) {
			_, err := client.CreateAlias(testDomain, "pulumi-test", "another@example.com")
			require.Error(t, err)
			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			assert.True(t, apiErr.IsAlreadyExists())
		})
		t.Run("Delete", func(t *testing.T) {
			err := client.DeleteAlias(testDomain, "pulumi-test")
			require.NoError(t, err)
		})
		t.Run("Delete_AlreadyGone", func(t *testing.T) {
			err := client.DeleteAlias(testDomain, "pulumi-test")
			require.Error(t, err)
			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			assert.True(t, apiErr.IsNotFound())
		})
		t.Run("Read_AfterDelete", func(t *testing.T) {
			_, err := client.GetAlias(testDomain, "pulumi-test")
			require.Error(t, err)
			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			assert.True(t, apiErr.IsNotFound())
		})
		t.Run("WildcardCreate", func(t *testing.T) {
			alias, err := client.CreateAlias(testDomain, "*", "catchall@example.com")
			require.NoError(t, err)
			assert.Equal(t, "*", alias.Alias)
		})
		t.Run("WildcardDelete", func(t *testing.T) {
			err := client.DeleteAlias(testDomain, "*")
			require.NoError(t, err)
		})
		t.Run("CleanupDomain", func(t *testing.T) {
			err := client.DeleteDomain(testDomain)
			require.NoError(t, err)
		})
	})

	t.Run("SmtpCredential", func(t *testing.T) {
		t.Run("SetupDomain", func(t *testing.T) {
			_, err := client.CreateDomain(testDomain, "")
			require.NoError(t, err)
		})
		t.Run("Create", func(t *testing.T) {
			cred, err := client.CreateSmtpCredential(testDomain, "pulumi-test-sender", "T3stP@ssw0rd!")
			require.NoError(t, err)
			assert.Equal(t, "pulumi-test-sender", cred.Username)
		})
		t.Run("Read", func(t *testing.T) {
			creds, err := client.ListSmtpCredentials(testDomain)
			require.NoError(t, err)
			found := false
			for _, c := range creds {
				if c.Username == "pulumi-test-sender" {
					found = true
					break
				}
			}
			assert.True(t, found, "credential should be found in list")
		})
		t.Run("Update_Password", func(t *testing.T) {
			err := client.UpdateSmtpCredential(testDomain, "pulumi-test-sender", "N3wP@ssw0rd!")
			require.NoError(t, err)
		})
		t.Run("Read_AfterUpdate", func(t *testing.T) {
			creds, err := client.ListSmtpCredentials(testDomain)
			require.NoError(t, err)
			found := false
			for _, c := range creds {
				if c.Username == "pulumi-test-sender" {
					found = true
					break
				}
			}
			assert.True(t, found, "credential should still exist after password update")
		})
		t.Run("CreateAgain_Adopt", func(t *testing.T) {
			_, err := client.CreateSmtpCredential(testDomain, "pulumi-test-sender", "AnotherP@ss!")
			require.Error(t, err)
			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			assert.True(t, apiErr.IsAlreadyExists())
		})
		t.Run("Delete", func(t *testing.T) {
			err := client.DeleteSmtpCredential(testDomain, "pulumi-test-sender")
			require.NoError(t, err)
		})
		t.Run("Delete_AlreadyGone", func(t *testing.T) {
			err := client.DeleteSmtpCredential(testDomain, "pulumi-test-sender")
			require.Error(t, err)
			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			assert.True(t, apiErr.IsNotFound())
		})
		t.Run("Read_AfterDelete", func(t *testing.T) {
			creds, err := client.ListSmtpCredentials(testDomain)
			require.NoError(t, err)
			for _, c := range creds {
				assert.NotEqual(t, "pulumi-test-sender", c.Username, "deleted credential should not appear in list")
			}
		})
		t.Run("CleanupDomain", func(t *testing.T) {
			err := client.DeleteDomain(testDomain)
			require.NoError(t, err)
		})
	})
}
