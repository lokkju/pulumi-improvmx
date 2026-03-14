package provider

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *ImprovMXClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := NewImprovMXClient("test-token")
	client.baseURL = server.URL
	return client
}

func jsonResponse(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func TestListDomains(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/domains", r.URL.Path)
		jsonResponse(w, 200, map[string]any{
			"success": true,
			"domains": []map[string]any{
				{"domain": "example.com", "active": true},
			},
		})
	})
	domains, err := client.ListDomains()
	require.NoError(t, err)
	assert.Len(t, domains, 1)
	assert.Equal(t, "example.com", domains[0].Domain)
}

func TestCreateDomain(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "example.com", body["domain"])
		jsonResponse(w, 200, map[string]any{
			"success": true,
			"domain":  map[string]any{"domain": "example.com", "active": false},
		})
	})
	domain, err := client.CreateDomain("example.com", "")
	require.NoError(t, err)
	assert.Equal(t, "example.com", domain.Domain)
}

func TestGetDomain(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/domains/example.com", r.URL.Path)
		jsonResponse(w, 200, map[string]any{
			"success": true,
			"domain":  map[string]any{"domain": "example.com", "active": true},
		})
	})
	domain, err := client.GetDomain("example.com")
	require.NoError(t, err)
	assert.True(t, domain.Active)
}

func TestDeleteDomain(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		jsonResponse(w, 200, map[string]any{"success": true})
	})
	err := client.DeleteDomain("example.com")
	require.NoError(t, err)
}

func TestCreateAlias(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/domains/example.com/aliases", r.URL.Path)
		jsonResponse(w, 200, map[string]any{
			"success": true,
			"alias":   map[string]any{"alias": "*", "forward": "user@gmail.com", "id": 123},
		})
	})
	alias, err := client.CreateAlias("example.com", "*", "user@gmail.com")
	require.NoError(t, err)
	assert.Equal(t, "*", alias.Alias)
	assert.Equal(t, "user@gmail.com", alias.Forward)
}

func TestUpdateAlias(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		jsonResponse(w, 200, map[string]any{
			"success": true,
			"alias":   map[string]any{"alias": "info", "forward": "new@gmail.com", "id": 123},
		})
	})
	alias, err := client.UpdateAlias("example.com", "info", "new@gmail.com")
	require.NoError(t, err)
	assert.Equal(t, "new@gmail.com", alias.Forward)
}

func TestCreateSmtpCredential(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		jsonResponse(w, 200, map[string]any{
			"success":    true,
			"credential": map[string]any{"username": "sender", "created": 1700000000},
		})
	})
	cred, err := client.CreateSmtpCredential("example.com", "sender", "pass123")
	require.NoError(t, err)
	assert.Equal(t, "sender", cred.Username)
}

func TestAPIError(t *testing.T) {
	// 401 returns a hard authentication error (not an APIError)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 401, map[string]any{
			"success": false,
			"errors":  map[string]string{"token": "Invalid API token"},
		})
	})
	_, err := client.ListDomains()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")

	// 400 returns an APIError
	client400 := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 400, map[string]any{
			"success": false,
			"errors":  map[string]string{"domain": "already registered"},
		})
	})
	_, err = client400.ListDomains()
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 400, apiErr.StatusCode)
}

func TestBasicAuth(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "api", user)
		assert.Equal(t, "test-token", pass)
		jsonResponse(w, 200, map[string]any{"success": true, "domains": []any{}})
	})
	_, err := client.ListDomains()
	require.NoError(t, err)
}

func TestCreateDomain_AlreadyExists(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 400, map[string]any{
			"success": false,
			"errors":  map[string]string{"domain": "Domain already registered"},
		})
	})
	_, err := client.CreateDomain("example.com", "")
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.True(t, apiErr.IsAlreadyExists())
}

func TestCreateAlias_AlreadyExists(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 400, map[string]any{
			"success": false,
			"errors":  map[string]string{"alias": "If you want to add multiple emails for an alias, update the existing one"},
		})
	})
	_, err := client.CreateAlias("example.com", "info", "user@example.com")
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.True(t, apiErr.IsAlreadyExists())
}

func TestGetDomain_NotFound(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 404, map[string]any{
			"success": false,
			"errors":  map[string]string{"domain": "Domain not found"},
		})
	})
	_, err := client.GetDomain("nonexistent.com")
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.True(t, apiErr.IsNotFound())
}

func TestGetAlias_NotFound(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 404, map[string]any{
			"success": false,
			"errors":  map[string]string{"alias": "Alias not found"},
		})
	})
	_, err := client.GetAlias("example.com", "nonexistent")
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.True(t, apiErr.IsNotFound())
}

func TestDeleteDomain_NotFound(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 404, map[string]any{
			"success": false,
			"errors":  map[string]string{"domain": "Domain not found"},
		})
	})
	err := client.DeleteDomain("nonexistent.com")
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.True(t, apiErr.IsNotFound())
}

func TestAuthFailure_401(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 401, map[string]any{
			"success": false,
			"errors":  map[string]string{"token": "Invalid API token"},
		})
	})
	_, err := client.ListDomains()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
	var apiErr *APIError
	assert.False(t, errors.As(err, &apiErr), "auth errors should not be APIError")
}

func TestAuthFailure_403(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 403, map[string]any{
			"success": false,
			"errors":  map[string]string{"token": "Forbidden"},
		})
	})
	_, err := client.ListDomains()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
	var apiErr *APIError
	assert.False(t, errors.As(err, &apiErr), "auth errors should not be APIError")
}

func TestUpdateDomain(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/domains/example.com", r.URL.Path)
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "admin@example.com", body["notification_email"])
		jsonResponse(w, 200, map[string]any{
			"success": true,
			"domain":  map[string]any{"domain": "example.com", "active": true, "notification_email": "admin@example.com"},
		})
	})
	domain, err := client.UpdateDomain("example.com", map[string]string{"notification_email": "admin@example.com"})
	require.NoError(t, err)
	assert.Equal(t, "admin@example.com", domain.NotificationEmail)
}

func TestListAliases(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/domains/example.com/aliases", r.URL.Path)
		jsonResponse(w, 200, map[string]any{
			"success": true,
			"aliases": []map[string]any{
				{"alias": "info", "forward": "user@gmail.com", "id": 1},
				{"alias": "*", "forward": "catch@gmail.com", "id": 2},
			},
		})
	})
	aliases, err := client.ListAliases("example.com")
	require.NoError(t, err)
	assert.Len(t, aliases, 2)
	assert.Equal(t, "info", aliases[0].Alias)
}

func TestDeleteAlias(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/domains/example.com/aliases/info", r.URL.Path)
		jsonResponse(w, 200, map[string]any{"success": true})
	})
	err := client.DeleteAlias("example.com", "info")
	require.NoError(t, err)
}

func TestListSmtpCredentials(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/domains/example.com/credentials", r.URL.Path)
		jsonResponse(w, 200, map[string]any{
			"success": true,
			"credentials": []map[string]any{
				{"username": "sender", "created": 1700000000},
			},
		})
	})
	creds, err := client.ListSmtpCredentials("example.com")
	require.NoError(t, err)
	assert.Len(t, creds, 1)
	assert.Equal(t, "sender", creds[0].Username)
}

func TestUpdateSmtpCredential(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/domains/example.com/credentials/sender", r.URL.Path)
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "newpass", body["password"])
		jsonResponse(w, 200, map[string]any{"success": true})
	})
	err := client.UpdateSmtpCredential("example.com", "sender", "newpass")
	require.NoError(t, err)
}

func TestDeleteSmtpCredential(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/domains/example.com/credentials/sender", r.URL.Path)
		jsonResponse(w, 200, map[string]any{"success": true})
	})
	err := client.DeleteSmtpCredential("example.com", "sender")
	require.NoError(t, err)
}
