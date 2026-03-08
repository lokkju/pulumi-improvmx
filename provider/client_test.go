package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(handler http.HandlerFunc) *ImprovMXClient {
	server := httptest.NewServer(handler)
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
	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
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
	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
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
	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
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
	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		jsonResponse(w, 200, map[string]any{"success": true})
	})
	err := client.DeleteDomain("example.com")
	require.NoError(t, err)
}

func TestCreateAlias(t *testing.T) {
	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
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
	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
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
	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
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
	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 401, map[string]any{
			"success": false,
			"errors":  map[string]string{"token": "Invalid API token"},
		})
	})
	_, err := client.ListDomains()
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 401, apiErr.StatusCode)
}

func TestBasicAuth(t *testing.T) {
	client := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "api", user)
		assert.Equal(t, "test-token", pass)
		jsonResponse(w, 200, map[string]any{"success": true, "domains": []any{}})
	})
	_, err := client.ListDomains()
	require.NoError(t, err)
}
