# Comprehensive Test Coverage Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement four-layer test coverage (client unit, provider unit, live integration, Pulumi lifecycle) to prevent accidental resource deletion and verify all CRUD/error paths.

**Architecture:** Refactor `getClient()` to support base URL override via env var for testability. Build a stateful mock API server for provider unit tests. Live and lifecycle tests share a safety gate that refuses to run against domains with existing resources.

**Tech Stack:** Go 1.24+, testify, `net/http/httptest`, `pulumi-go-provider/integration`, `.env.local` for credentials.

**Spec:** `docs/superpowers/specs/2026-03-14-comprehensive-test-coverage-design.md`

---

## File Structure

| File | Responsibility |
|---|---|
| `provider/client.go` | Add `BaseURL` override support to `NewImprovMXClient` |
| `provider/domain.go` | Update `getClient()` to use `IMPROVMX_BASE_URL` env var |
| `provider/client_test.go` | Layer 1: expand with error-path tests, fix server leaks |
| `provider/mock_api_test.go` | Stateful mock API server for Layer 2 tests |
| `provider/provider_test.go` | Layer 2: provider CRUD + Diff unit tests |
| `provider/live_test.go` | Layer 3: live integration tests with safety gate |
| `provider/lifecycle_test.go` | Layer 4: Pulumi lifecycle tests |
| `provider/integration_test.go` | DELETE (replaced by `live_test.go`) |
| `.env.sample` | Document required env vars |
| `.gitignore` | Add `.env.local` |
| `Makefile` | Replace `test_integration` with `test_live` |
| `CLAUDE.md` | Update testing docs |

---

## Chunk 1: Infrastructure (Tasks 1-3)

### Task 1: Refactor getClient() for testability

**Files:**
- Modify: `provider/domain.go:41-51`

- [ ] **Step 1: Update `getClient()` to check env vars first, safely fall back to Pulumi config**

`infer.GetConfig` panics when called with a bare `context.Background()` (no Pulumi runtime context). To support unit testing with `context.Background()`, check the env var first, and wrap the `infer.GetConfig` call in a recover.

```go
func getClient(ctx context.Context) (*ImprovMXClient, error) {
	// Check env var first — this path is always safe and supports unit testing
	// without a full Pulumi provider context.
	token := os.Getenv("IMPROVMX_API_TOKEN")
	if token == "" {
		// Try Pulumi config (may panic if ctx is not a Pulumi context)
		func() {
			defer func() { recover() }()
			config := infer.GetConfig[ProviderConfig](ctx)
			token = config.ApiToken
		}()
	}
	if token == "" {
		return nil, fmt.Errorf("ImprovMX API token not configured: set improvmx:apiToken in Pulumi config or IMPROVMX_API_TOKEN env var")
	}
	client := NewImprovMXClient(token)
	if baseURL := os.Getenv("IMPROVMX_BASE_URL"); baseURL != "" {
		client.baseURL = baseURL
	}
	return client, nil
}
```

- [ ] **Step 2: Run existing tests to verify no regression**

Run: `go test -v -count=1 ./provider/...`
Expected: All existing tests PASS.

- [ ] **Step 3: Commit**

```bash
git add provider/domain.go
git commit -m "refactor: support IMPROVMX_BASE_URL override in getClient for testability"
```

### Task 2: Fix client_test.go server lifecycle leaks and add error-path tests

**Files:**
- Modify: `provider/client_test.go`

- [ ] **Step 1: Fix `newTestClient` to register cleanup**

Replace the existing `newTestClient` function:

```go
func newTestClient(t *testing.T, handler http.HandlerFunc) *ImprovMXClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := NewImprovMXClient("test-token")
	client.baseURL = server.URL
	return client
}
```

Update all existing callers from `newTestClient(func(...))` to `newTestClient(t, func(...))`.

- [ ] **Step 2: Run existing tests to verify refactor**

Run: `go test -v -count=1 -run 'TestListDomains|TestCreateDomain|TestGetDomain|TestDeleteDomain|TestCreateAlias|TestUpdateAlias|TestCreateSmtpCredential|TestAPIError|TestBasicAuth' ./provider/...`
Expected: All 9 existing tests PASS.

- [ ] **Step 3: Add `IsAlreadyExists` tests**

```go
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
```

- [ ] **Step 4: Add `IsNotFound` tests**

```go
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
```

- [ ] **Step 5: Add auth failure tests**

```go
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
```

Add `"errors"` to the import list.

- [ ] **Step 6: Add remaining CRUD method tests**

```go
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
```

- [ ] **Step 7: Run all client tests**

Run: `go test -v -count=1 -run 'Test(ListDomains|CreateDomain|GetDomain|DeleteDomain|CreateAlias|UpdateAlias|CreateSmtpCredential|APIError|BasicAuth|AlreadyExists|NotFound|AuthFailure|UpdateDomain|ListAliases|DeleteAlias|ListSmtpCredentials|UpdateSmtpCredential|DeleteSmtpCredential)' ./provider/...`
Expected: All tests PASS (9 existing + 13 new = 22 total).

- [ ] **Step 8: Commit**

```bash
git add provider/client_test.go
git commit -m "test: expand client unit tests with error paths, fix server lifecycle leaks"
```

### Task 3: Build config and Makefile changes

**Files:**
- Modify: `.gitignore`
- Create: `.env.sample`
- Modify: `Makefile`

- [ ] **Step 1: Add `.env.local` to `.gitignore`**

Append to `.gitignore`:
```
.env.local
```

- [ ] **Step 2: Create `.env.sample`**

```
IMPROVMX_API_TOKEN=
IMPROVMX_INTEGRATION_TEST_DOMAIN=
```

- [ ] **Step 3: Update Makefile — replace `test_integration` with `test_live`**

Replace the `test_integration` target:

```makefile
.PHONY: test_live
test_live:
	set -a && [ -f .env.local ] && . .env.local || true; set +a; \
	IMPROVMX_LIVE_TEST=1 go test -v -count=1 -run 'TestLive|TestLifecycle' -timeout 10m ./provider/...
```

- [ ] **Step 4: Update CLAUDE.md testing section**

Replace the "Testing Layers" section and update the build commands to reference `test_live` instead of `test_integration`, and update the env var name to `IMPROVMX_INTEGRATION_TEST_DOMAIN`.

- [ ] **Step 5: Commit**

```bash
git add .gitignore .env.sample Makefile CLAUDE.md
git commit -m "chore: add .env.sample, update Makefile and docs for live test infrastructure"
```

---

## Chunk 2: Provider Unit Tests (Tasks 4-5)

### Task 4: Build stateful mock API server

**Files:**
- Create: `provider/mock_api_test.go`

- [ ] **Step 1: Write the mock API server**

This is a test-only file that implements an in-memory ImprovMX API. It tracks domains, aliases, and credentials in maps, and returns proper error responses for duplicates and not-found cases.

```go
package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockDomain struct {
	Domain            string `json:"domain"`
	Display           string `json:"display"`
	Active            bool   `json:"active"`
	NotificationEmail string `json:"notification_email,omitempty"`
	Webhook           string `json:"webhook,omitempty"`
}

type mockAlias struct {
	ID      int    `json:"id"`
	Alias   string `json:"alias"`
	Forward string `json:"forward"`
}

type mockCredential struct {
	Username string `json:"username"`
	Created  int64  `json:"created"`
}

type mockAPIState struct {
	mu          sync.Mutex
	domains     map[string]*mockDomain
	aliases     map[string]map[string]*mockAlias // domain -> alias -> info
	credentials map[string]map[string]*mockCredential // domain -> username -> info
	nextAliasID int
}

func newMockAPIServer(t *testing.T) (*httptest.Server, *mockAPIState) {
	t.Helper()
	state := &mockAPIState{
		domains:     make(map[string]*mockDomain),
		aliases:     make(map[string]map[string]*mockAlias),
		credentials: make(map[string]map[string]*mockCredential),
		nextAliasID: 1,
	}

	mux := http.NewServeMux()

	// POST /domains
	mux.HandleFunc("POST /domains", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		domainName := body["domain"]

		state.mu.Lock()
		defer state.mu.Unlock()

		if _, exists := state.domains[domainName]; exists {
			jsonResponse(w, 400, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "Domain already registered"},
			})
			return
		}

		d := &mockDomain{
			Domain:            domainName,
			Display:           domainName,
			Active:            false,
			NotificationEmail: body["notification_email"],
		}
		state.domains[domainName] = d
		state.aliases[domainName] = make(map[string]*mockAlias)
		state.credentials[domainName] = make(map[string]*mockCredential)
		jsonResponse(w, 200, map[string]any{"success": true, "domain": d})
	})

	// GET /domains/{domain}
	mux.HandleFunc("GET /domains/{domain}", func(w http.ResponseWriter, r *http.Request) {
		domainName := r.PathValue("domain")

		// Exclude sub-resource paths
		if strings.Contains(r.URL.Path, "/aliases") || strings.Contains(r.URL.Path, "/credentials") {
			http.NotFound(w, r)
			return
		}

		state.mu.Lock()
		defer state.mu.Unlock()

		d, exists := state.domains[domainName]
		if !exists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "Domain not found"},
			})
			return
		}
		jsonResponse(w, 200, map[string]any{"success": true, "domain": d})
	})

	// PUT /domains/{domain}
	mux.HandleFunc("PUT /domains/{domain}", func(w http.ResponseWriter, r *http.Request) {
		domainName := r.PathValue("domain")

		state.mu.Lock()
		defer state.mu.Unlock()

		d, exists := state.domains[domainName]
		if !exists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "Domain not found"},
			})
			return
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if v, ok := body["notification_email"]; ok {
			d.NotificationEmail = v
		}
		if v, ok := body["webhook"]; ok {
			d.Webhook = v
		}
		jsonResponse(w, 200, map[string]any{"success": true, "domain": d})
	})

	// DELETE /domains/{domain}
	mux.HandleFunc("DELETE /domains/{domain}", func(w http.ResponseWriter, r *http.Request) {
		domainName := r.PathValue("domain")

		state.mu.Lock()
		defer state.mu.Unlock()

		if _, exists := state.domains[domainName]; !exists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "Domain not found"},
			})
			return
		}
		delete(state.domains, domainName)
		delete(state.aliases, domainName)
		delete(state.credentials, domainName)
		jsonResponse(w, 200, map[string]any{"success": true})
	})

	// POST /domains/{domain}/aliases
	mux.HandleFunc("POST /domains/{domain}/aliases", func(w http.ResponseWriter, r *http.Request) {
		domainName := r.PathValue("domain")
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)

		state.mu.Lock()
		defer state.mu.Unlock()

		domainAliases, exists := state.aliases[domainName]
		if !exists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "Domain not found"},
			})
			return
		}

		aliasName := body["alias"]
		if _, exists := domainAliases[aliasName]; exists {
			jsonResponse(w, 400, map[string]any{
				"success": false,
				"errors":  map[string]string{"alias": "If you want to add multiple emails for an alias, update the existing one and separate them by a comma."},
			})
			return
		}

		a := &mockAlias{
			ID:      state.nextAliasID,
			Alias:   aliasName,
			Forward: body["forward"],
		}
		state.nextAliasID++
		domainAliases[aliasName] = a
		jsonResponse(w, 200, map[string]any{"success": true, "alias": a})
	})

	// GET /domains/{domain}/aliases/{alias}
	mux.HandleFunc("GET /domains/{domain}/aliases/{alias}", func(w http.ResponseWriter, r *http.Request) {
		domainName := r.PathValue("domain")
		aliasName := r.PathValue("alias")

		state.mu.Lock()
		defer state.mu.Unlock()

		domainAliases, exists := state.aliases[domainName]
		if !exists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "Domain not found"},
			})
			return
		}

		a, exists := domainAliases[aliasName]
		if !exists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"alias": "Alias not found"},
			})
			return
		}
		jsonResponse(w, 200, map[string]any{"success": true, "alias": a})
	})

	// GET /domains/{domain}/aliases
	mux.HandleFunc("GET /domains/{domain}/aliases", func(w http.ResponseWriter, r *http.Request) {
		domainName := r.PathValue("domain")

		state.mu.Lock()
		defer state.mu.Unlock()

		domainAliases, exists := state.aliases[domainName]
		if !exists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "Domain not found"},
			})
			return
		}

		var list []*mockAlias
		for _, a := range domainAliases {
			list = append(list, a)
		}
		jsonResponse(w, 200, map[string]any{"success": true, "aliases": list})
	})

	// PUT /domains/{domain}/aliases/{alias}
	mux.HandleFunc("PUT /domains/{domain}/aliases/{alias}", func(w http.ResponseWriter, r *http.Request) {
		domainName := r.PathValue("domain")
		aliasName := r.PathValue("alias")
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)

		state.mu.Lock()
		defer state.mu.Unlock()

		domainAliases, exists := state.aliases[domainName]
		if !exists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "Domain not found"},
			})
			return
		}

		a, exists := domainAliases[aliasName]
		if !exists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"alias": "Alias not found"},
			})
			return
		}
		a.Forward = body["forward"]
		jsonResponse(w, 200, map[string]any{"success": true, "alias": a})
	})

	// DELETE /domains/{domain}/aliases/{alias}
	mux.HandleFunc("DELETE /domains/{domain}/aliases/{alias}", func(w http.ResponseWriter, r *http.Request) {
		domainName := r.PathValue("domain")
		aliasName := r.PathValue("alias")

		state.mu.Lock()
		defer state.mu.Unlock()

		domainAliases, exists := state.aliases[domainName]
		if !exists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "Domain not found"},
			})
			return
		}
		if _, exists := domainAliases[aliasName]; !exists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"alias": "Alias not found"},
			})
			return
		}
		delete(domainAliases, aliasName)
		jsonResponse(w, 200, map[string]any{"success": true})
	})

	// POST /domains/{domain}/credentials
	mux.HandleFunc("POST /domains/{domain}/credentials", func(w http.ResponseWriter, r *http.Request) {
		domainName := r.PathValue("domain")
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)

		state.mu.Lock()
		defer state.mu.Unlock()

		domainCreds, exists := state.credentials[domainName]
		if !exists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "Domain not found"},
			})
			return
		}

		username := body["username"]
		if _, exists := domainCreds[username]; exists {
			jsonResponse(w, 400, map[string]any{
				"success": false,
				"errors":  map[string]string{"username": "Credential already exists"},
			})
			return
		}

		c := &mockCredential{
			Username: username,
			Created:  time.Now().Unix(),
		}
		domainCreds[username] = c
		jsonResponse(w, 200, map[string]any{"success": true, "credential": c})
	})

	// GET /domains/{domain}/credentials
	mux.HandleFunc("GET /domains/{domain}/credentials", func(w http.ResponseWriter, r *http.Request) {
		domainName := r.PathValue("domain")

		state.mu.Lock()
		defer state.mu.Unlock()

		domainCreds, exists := state.credentials[domainName]
		if !exists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "Domain not found"},
			})
			return
		}

		var list []*mockCredential
		for _, c := range domainCreds {
			list = append(list, c)
		}
		jsonResponse(w, 200, map[string]any{"success": true, "credentials": list})
	})

	// PUT /domains/{domain}/credentials/{username}
	mux.HandleFunc("PUT /domains/{domain}/credentials/{username}", func(w http.ResponseWriter, r *http.Request) {
		domainName := r.PathValue("domain")
		username := r.PathValue("username")

		state.mu.Lock()
		defer state.mu.Unlock()

		domainCreds, exists := state.credentials[domainName]
		if !exists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "Domain not found"},
			})
			return
		}
		if _, exists := domainCreds[username]; !exists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"credential": "Credential not found"},
			})
			return
		}
		// Password update is accepted but we don't store it
		jsonResponse(w, 200, map[string]any{"success": true})
	})

	// DELETE /domains/{domain}/credentials/{username}
	mux.HandleFunc("DELETE /domains/{domain}/credentials/{username}", func(w http.ResponseWriter, r *http.Request) {
		domainName := r.PathValue("domain")
		username := r.PathValue("username")

		state.mu.Lock()
		defer state.mu.Unlock()

		domainCreds, exists := state.credentials[domainName]
		if !exists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "Domain not found"},
			})
			return
		}
		if _, exists := domainCreds[username]; !exists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"credential": "Credential not found"},
			})
			return
		}
		delete(domainCreds, username)
		jsonResponse(w, 200, map[string]any{"success": true})
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, state
}

// newMockClient creates an ImprovMXClient pointing at the mock server.
func newMockClient(t *testing.T, server *httptest.Server) *ImprovMXClient {
	t.Helper()
	client := NewImprovMXClient("mock-token")
	client.baseURL = server.URL
	return client
}
```

- [ ] **Step 2: Run build to verify compilation**

Run: `go build ./provider/...`
Expected: No errors (unused imports from `fmt` and `time` are used).

- [ ] **Step 3: Commit**

```bash
git add provider/mock_api_test.go
git commit -m "test: add stateful mock API server for provider unit tests"
```

### Task 5: Write provider unit tests

**Files:**
- Create: `provider/provider_test.go`

- [ ] **Step 1: Write Domain CRUD and Diff tests**

```go
package provider

import (
	"context"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupMockEnv sets IMPROVMX_BASE_URL and IMPROVMX_API_TOKEN for provider tests.
// Returns a cleanup function.
func setupMockEnv(t *testing.T, baseURL string) {
	t.Helper()
	t.Setenv("IMPROVMX_BASE_URL", baseURL)
	t.Setenv("IMPROVMX_API_TOKEN", "mock-token")
}

// --- Domain Tests ---

func TestDomainCreate(t *testing.T) {
	server, _ := newMockAPIServer(t)
	setupMockEnv(t, server.URL)

	resp, err := Domain{}.Create(context.Background(), infer.CreateRequest[DomainArgs]{
		Inputs: DomainArgs{Domain: "test.com"},
	})
	require.NoError(t, err)
	assert.Equal(t, "test.com", resp.ID)
	assert.Equal(t, "test.com", resp.Output.Domain)
	assert.Equal(t, "test.com", resp.Output.Display)
}

func TestDomainCreate_AdoptExisting(t *testing.T) {
	server, state := newMockAPIServer(t)
	setupMockEnv(t, server.URL)

	// Pre-populate the domain
	state.mu.Lock()
	state.domains["test.com"] = &mockDomain{Domain: "test.com", Display: "test.com", Active: false}
	state.aliases["test.com"] = make(map[string]*mockAlias)
	state.credentials["test.com"] = make(map[string]*mockCredential)
	state.mu.Unlock()

	email := "admin@test.com"
	webhook := "https://hooks.test.com/notify"
	resp, err := Domain{}.Create(context.Background(), infer.CreateRequest[DomainArgs]{
		Inputs: DomainArgs{
			Domain:            "test.com",
			NotificationEmail: &email,
			Webhook:           &webhook,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "test.com", resp.ID)
	assert.Equal(t, &email, resp.Output.NotificationEmail)
	assert.Equal(t, &webhook, resp.Output.Webhook)
}

func TestDomainRead(t *testing.T) {
	server, state := newMockAPIServer(t)
	setupMockEnv(t, server.URL)

	state.mu.Lock()
	state.domains["test.com"] = &mockDomain{Domain: "test.com", Display: "test.com", Active: true, NotificationEmail: "admin@test.com"}
	state.mu.Unlock()

	resp, err := Domain{}.Read(context.Background(), infer.ReadRequest[DomainArgs, DomainState]{ID: "test.com"})
	require.NoError(t, err)
	assert.Equal(t, "test.com", resp.ID)
	assert.True(t, resp.State.Active)
	require.NotNil(t, resp.Inputs.NotificationEmail)
	assert.Equal(t, "admin@test.com", *resp.Inputs.NotificationEmail)
}

func TestDomainRead_NotFound(t *testing.T) {
	server, _ := newMockAPIServer(t)
	setupMockEnv(t, server.URL)

	resp, err := Domain{}.Read(context.Background(), infer.ReadRequest[DomainArgs, DomainState]{ID: "gone.com"})
	require.NoError(t, err)
	assert.Equal(t, "", resp.ID)
}

func TestDomainUpdate(t *testing.T) {
	server, state := newMockAPIServer(t)
	setupMockEnv(t, server.URL)

	state.mu.Lock()
	state.domains["test.com"] = &mockDomain{Domain: "test.com", Display: "test.com", Active: true}
	state.mu.Unlock()

	email := "new@test.com"
	resp, err := Domain{}.Update(context.Background(), infer.UpdateRequest[DomainArgs, DomainState]{
		ID:     "test.com",
		Inputs: DomainArgs{Domain: "test.com", NotificationEmail: &email},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Output.NotificationEmail)
	assert.Equal(t, "new@test.com", *resp.Output.NotificationEmail)
}

func TestDomainDelete(t *testing.T) {
	server, state := newMockAPIServer(t)
	setupMockEnv(t, server.URL)

	state.mu.Lock()
	state.domains["test.com"] = &mockDomain{Domain: "test.com"}
	state.aliases["test.com"] = make(map[string]*mockAlias)
	state.credentials["test.com"] = make(map[string]*mockCredential)
	state.mu.Unlock()

	_, err := Domain{}.Delete(context.Background(), infer.DeleteRequest[DomainState]{ID: "test.com"})
	require.NoError(t, err)
}

func TestDomainDelete_AlreadyGone(t *testing.T) {
	server, _ := newMockAPIServer(t)
	setupMockEnv(t, server.URL)

	_, err := Domain{}.Delete(context.Background(), infer.DeleteRequest[DomainState]{ID: "gone.com"})
	require.NoError(t, err, "deleting non-existent domain should succeed (idempotent)")
}

func TestDomainDiff_NoChange(t *testing.T) {
	email := "admin@test.com"
	resp, err := Domain{}.Diff(context.Background(), infer.DiffRequest[DomainArgs, DomainState]{
		Inputs: DomainArgs{Domain: "test.com", NotificationEmail: &email},
		State:  DomainState{DomainArgs: DomainArgs{Domain: "test.com", NotificationEmail: &email}},
	})
	require.NoError(t, err)
	assert.False(t, resp.HasChanges)
	assert.False(t, resp.DeleteBeforeReplace)
}

func TestDomainDiff_UpdateOnly(t *testing.T) {
	oldEmail := "old@test.com"
	newEmail := "new@test.com"
	resp, err := Domain{}.Diff(context.Background(), infer.DiffRequest[DomainArgs, DomainState]{
		Inputs: DomainArgs{Domain: "test.com", NotificationEmail: &newEmail},
		State:  DomainState{DomainArgs: DomainArgs{Domain: "test.com", NotificationEmail: &oldEmail}},
	})
	require.NoError(t, err)
	assert.True(t, resp.HasChanges)
	assert.False(t, resp.DeleteBeforeReplace, "updating notificationEmail should NOT trigger replace")
	assert.Equal(t, p.Update, resp.DetailedDiff["notificationEmail"].Kind)
}

func TestDomainDiff_Replace(t *testing.T) {
	resp, err := Domain{}.Diff(context.Background(), infer.DiffRequest[DomainArgs, DomainState]{
		Inputs: DomainArgs{Domain: "new.com"},
		State:  DomainState{DomainArgs: DomainArgs{Domain: "old.com"}},
	})
	require.NoError(t, err)
	assert.True(t, resp.HasChanges)
	assert.True(t, resp.DeleteBeforeReplace, "changing domain should trigger DeleteBeforeReplace")
	assert.Equal(t, p.UpdateReplace, resp.DetailedDiff["domain"].Kind)
}
```

- [ ] **Step 2: Run Domain tests**

Run: `go test -v -count=1 -run 'TestDomain' ./provider/...`
Expected: All 10 tests PASS.

- [ ] **Step 3: Write EmailAlias tests**

```go
// --- EmailAlias Tests ---

func TestAliasCreate(t *testing.T) {
	server, state := newMockAPIServer(t)
	setupMockEnv(t, server.URL)

	state.mu.Lock()
	state.domains["test.com"] = &mockDomain{Domain: "test.com"}
	state.aliases["test.com"] = make(map[string]*mockAlias)
	state.mu.Unlock()

	resp, err := EmailAlias{}.Create(context.Background(), infer.CreateRequest[EmailAliasArgs]{
		Inputs: EmailAliasArgs{Domain: "test.com", Alias: "info", Forward: "user@gmail.com"},
	})
	require.NoError(t, err)
	assert.Equal(t, "test.com/info", resp.ID)
	assert.Equal(t, "user@gmail.com", resp.Output.Forward)
}

func TestAliasCreate_AdoptAndUpdate(t *testing.T) {
	server, state := newMockAPIServer(t)
	setupMockEnv(t, server.URL)

	state.mu.Lock()
	state.domains["test.com"] = &mockDomain{Domain: "test.com"}
	state.aliases["test.com"] = map[string]*mockAlias{
		"info": {ID: 1, Alias: "info", Forward: "old@gmail.com"},
	}
	state.mu.Unlock()

	resp, err := EmailAlias{}.Create(context.Background(), infer.CreateRequest[EmailAliasArgs]{
		Inputs: EmailAliasArgs{Domain: "test.com", Alias: "info", Forward: "new@gmail.com"},
	})
	require.NoError(t, err)
	assert.Equal(t, "test.com/info", resp.ID)
	assert.Equal(t, "new@gmail.com", resp.Output.Forward, "forward should be updated during adopt")
}

func TestAliasRead_NotFound(t *testing.T) {
	server, state := newMockAPIServer(t)
	setupMockEnv(t, server.URL)

	state.mu.Lock()
	state.domains["test.com"] = &mockDomain{Domain: "test.com"}
	state.aliases["test.com"] = make(map[string]*mockAlias)
	state.mu.Unlock()

	resp, err := EmailAlias{}.Read(context.Background(), infer.ReadRequest[EmailAliasArgs, EmailAliasState]{ID: "test.com/gone"})
	require.NoError(t, err)
	assert.Equal(t, "", resp.ID)
}

func TestAliasDelete_AlreadyGone(t *testing.T) {
	server, state := newMockAPIServer(t)
	setupMockEnv(t, server.URL)

	state.mu.Lock()
	state.domains["test.com"] = &mockDomain{Domain: "test.com"}
	state.aliases["test.com"] = make(map[string]*mockAlias)
	state.mu.Unlock()

	_, err := EmailAlias{}.Delete(context.Background(), infer.DeleteRequest[EmailAliasState]{
		ID:    "test.com/gone",
		State: EmailAliasState{EmailAliasArgs: EmailAliasArgs{Domain: "test.com", Alias: "gone"}},
	})
	require.NoError(t, err, "deleting non-existent alias should succeed (idempotent)")
}

func TestAliasDiff_UpdateForward(t *testing.T) {
	resp, err := EmailAlias{}.Diff(context.Background(), infer.DiffRequest[EmailAliasArgs, EmailAliasState]{
		Inputs: EmailAliasArgs{Domain: "test.com", Alias: "info", Forward: "new@gmail.com"},
		State:  EmailAliasState{EmailAliasArgs: EmailAliasArgs{Domain: "test.com", Alias: "info", Forward: "old@gmail.com"}},
	})
	require.NoError(t, err)
	assert.True(t, resp.HasChanges)
	assert.False(t, resp.DeleteBeforeReplace, "updating forward should NOT trigger replace")
	assert.Equal(t, p.Update, resp.DetailedDiff["forward"].Kind)
}

func TestAliasDiff_ReplaceAlias(t *testing.T) {
	resp, err := EmailAlias{}.Diff(context.Background(), infer.DiffRequest[EmailAliasArgs, EmailAliasState]{
		Inputs: EmailAliasArgs{Domain: "test.com", Alias: "support", Forward: "user@gmail.com"},
		State:  EmailAliasState{EmailAliasArgs: EmailAliasArgs{Domain: "test.com", Alias: "info", Forward: "user@gmail.com"}},
	})
	require.NoError(t, err)
	assert.True(t, resp.HasChanges)
	assert.True(t, resp.DeleteBeforeReplace, "changing alias should trigger replace")
	assert.Equal(t, p.UpdateReplace, resp.DetailedDiff["alias"].Kind)
}

func TestAliasDiff_ReplaceDomain(t *testing.T) {
	resp, err := EmailAlias{}.Diff(context.Background(), infer.DiffRequest[EmailAliasArgs, EmailAliasState]{
		Inputs: EmailAliasArgs{Domain: "new.com", Alias: "info", Forward: "user@gmail.com"},
		State:  EmailAliasState{EmailAliasArgs: EmailAliasArgs{Domain: "old.com", Alias: "info", Forward: "user@gmail.com"}},
	})
	require.NoError(t, err)
	assert.True(t, resp.HasChanges)
	assert.True(t, resp.DeleteBeforeReplace, "changing domain should trigger replace")
	assert.Equal(t, p.UpdateReplace, resp.DetailedDiff["domain"].Kind)
}
```

- [ ] **Step 4: Run EmailAlias tests**

Run: `go test -v -count=1 -run 'TestAlias' ./provider/...`
Expected: All 7 tests PASS.

- [ ] **Step 5: Write SmtpCredential tests**

```go
// --- SmtpCredential Tests ---

func TestCredentialCreate(t *testing.T) {
	server, state := newMockAPIServer(t)
	setupMockEnv(t, server.URL)

	state.mu.Lock()
	state.domains["test.com"] = &mockDomain{Domain: "test.com"}
	state.credentials["test.com"] = make(map[string]*mockCredential)
	state.mu.Unlock()

	resp, err := SmtpCredential{}.Create(context.Background(), infer.CreateRequest[SmtpCredentialArgs]{
		Inputs: SmtpCredentialArgs{Domain: "test.com", Username: "sender", Password: "pass123"},
	})
	require.NoError(t, err)
	assert.Equal(t, "test.com/sender", resp.ID)
	assert.Equal(t, "sender", resp.Output.Username)
	assert.NotZero(t, resp.Output.Created)
}

func TestCredentialCreate_AdoptExisting(t *testing.T) {
	server, state := newMockAPIServer(t)
	setupMockEnv(t, server.URL)

	state.mu.Lock()
	state.domains["test.com"] = &mockDomain{Domain: "test.com"}
	state.credentials["test.com"] = map[string]*mockCredential{
		"sender": {Username: "sender", Created: 1700000000},
	}
	state.mu.Unlock()

	resp, err := SmtpCredential{}.Create(context.Background(), infer.CreateRequest[SmtpCredentialArgs]{
		Inputs: SmtpCredentialArgs{Domain: "test.com", Username: "sender", Password: "newpass"},
	})
	require.NoError(t, err)
	assert.Equal(t, "test.com/sender", resp.ID)
	assert.Equal(t, "sender", resp.Output.Username)
}

func TestCredentialRead_NotFound(t *testing.T) {
	server, state := newMockAPIServer(t)
	setupMockEnv(t, server.URL)

	state.mu.Lock()
	state.domains["test.com"] = &mockDomain{Domain: "test.com"}
	state.credentials["test.com"] = make(map[string]*mockCredential)
	state.mu.Unlock()

	resp, err := SmtpCredential{}.Read(context.Background(), infer.ReadRequest[SmtpCredentialArgs, SmtpCredentialState]{
		ID:    "test.com/gone",
		State: SmtpCredentialState{SmtpCredentialArgs: SmtpCredentialArgs{Password: "old"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "", resp.ID)
}

func TestCredentialRead_DomainGone(t *testing.T) {
	server, _ := newMockAPIServer(t)
	setupMockEnv(t, server.URL)

	resp, err := SmtpCredential{}.Read(context.Background(), infer.ReadRequest[SmtpCredentialArgs, SmtpCredentialState]{
		ID:    "gone.com/sender",
		State: SmtpCredentialState{SmtpCredentialArgs: SmtpCredentialArgs{Password: "old"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "", resp.ID, "should signal deletion when domain is gone")
}

func TestCredentialDelete_AlreadyGone(t *testing.T) {
	server, state := newMockAPIServer(t)
	setupMockEnv(t, server.URL)

	state.mu.Lock()
	state.domains["test.com"] = &mockDomain{Domain: "test.com"}
	state.credentials["test.com"] = make(map[string]*mockCredential)
	state.mu.Unlock()

	_, err := SmtpCredential{}.Delete(context.Background(), infer.DeleteRequest[SmtpCredentialState]{
		ID:    "test.com/gone",
		State: SmtpCredentialState{SmtpCredentialArgs: SmtpCredentialArgs{Domain: "test.com", Username: "gone"}},
	})
	require.NoError(t, err, "deleting non-existent credential should succeed (idempotent)")
}

func TestCredentialDiff_UpdatePassword(t *testing.T) {
	resp, err := SmtpCredential{}.Diff(context.Background(), infer.DiffRequest[SmtpCredentialArgs, SmtpCredentialState]{
		Inputs: SmtpCredentialArgs{Domain: "test.com", Username: "sender", Password: "newpass"},
		State:  SmtpCredentialState{SmtpCredentialArgs: SmtpCredentialArgs{Domain: "test.com", Username: "sender", Password: "oldpass"}},
	})
	require.NoError(t, err)
	assert.True(t, resp.HasChanges)
	assert.False(t, resp.DeleteBeforeReplace, "updating password should NOT trigger replace")
	assert.Equal(t, p.Update, resp.DetailedDiff["password"].Kind)
}

func TestCredentialDiff_ReplaceUsername(t *testing.T) {
	resp, err := SmtpCredential{}.Diff(context.Background(), infer.DiffRequest[SmtpCredentialArgs, SmtpCredentialState]{
		Inputs: SmtpCredentialArgs{Domain: "test.com", Username: "newsender", Password: "pass"},
		State:  SmtpCredentialState{SmtpCredentialArgs: SmtpCredentialArgs{Domain: "test.com", Username: "oldsender", Password: "pass"}},
	})
	require.NoError(t, err)
	assert.True(t, resp.HasChanges)
	assert.True(t, resp.DeleteBeforeReplace, "changing username should trigger replace")
	assert.Equal(t, p.UpdateReplace, resp.DetailedDiff["username"].Kind)
}
```

- [ ] **Step 6: Run all provider tests**

Run: `go test -v -count=1 -run 'TestDomain|TestAlias|TestCredential' ./provider/...`
Expected: All 24 provider tests PASS.

- [ ] **Step 7: Commit**

```bash
git add provider/provider_test.go
git commit -m "test: add provider unit tests for CRUD, adopt, idempotent delete, and Diff semantics"
```

---

## Chunk 3: Live and Lifecycle Tests (Tasks 6-8)

### Task 6: Write test helpers for live tests

**Files:**
- Create: `provider/live_test.go`

- [ ] **Step 1: Write safety gate and helpers**

```go
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

// safetyGate verifies the test domain is clean (no aliases, no credentials).
// Logs a warning and refuses to run if the domain has existing resources.
func safetyGate(t *testing.T, client *ImprovMXClient, domain string) {
	t.Helper()

	fmt.Printf("\n")
	fmt.Printf("========================================================================\n")
	fmt.Printf("WARNING: Live tests will CREATE and DELETE resources on domain %s.\n", domain)
	fmt.Printf("         The domain itself WILL be destroyed.\n")
	fmt.Printf("========================================================================\n")
	fmt.Printf("\n")

	// Check if domain exists
	d, err := client.GetDomain(domain)
	if err != nil {
		// Domain doesn't exist — that's fine, tests will create it
		if apiErr, ok := err.(*APIError); ok && apiErr.IsNotFound() {
			return
		}
		t.Fatalf("Failed to check domain %s: %v", domain, err)
	}

	_ = d

	// Check for existing aliases
	aliases, err := client.ListAliases(domain)
	if err != nil {
		t.Fatalf("Failed to list aliases for %s: %v", domain, err)
	}
	if len(aliases) > 0 {
		t.Fatalf("Refusing to run: domain %s has %d existing aliases. Use a clean test domain.", domain, len(aliases))
	}

	// Check for existing SMTP credentials
	creds, err := client.ListSmtpCredentials(domain)
	if err != nil {
		t.Fatalf("Failed to list credentials for %s: %v", domain, err)
	}
	if len(creds) > 0 {
		t.Fatalf("Refusing to run: domain %s has %d existing SMTP credentials. Use a clean test domain.", domain, len(creds))
	}

	// Clean slate: delete the domain so tests start fresh
	_ = client.DeleteDomain(domain)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./provider/...`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add provider/live_test.go
git commit -m "test: add live test safety gate and helpers"
```

### Task 7: Write live integration test subtests

**Files:**
- Modify: `provider/live_test.go`

- [ ] **Step 1: Write `TestLive` with Domain subtests**

Append to `provider/live_test.go`:

```go
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

			// Verify we can still read it
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
```

- [ ] **Step 2: Delete old `integration_test.go`**

```bash
rm provider/integration_test.go
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./provider/...`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add provider/live_test.go
git rm provider/integration_test.go
git commit -m "test: add comprehensive live integration tests with safety gate, replace old integration tests"
```

### Task 8: Write Pulumi lifecycle tests

**Files:**
- Create: `provider/lifecycle_test.go`

- [ ] **Step 1: Write lifecycle test file**

```go
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

func TestLifecycleAlias_CreateUpdateDelete(t *testing.T) {
	skipIfNoLiveAPI(t)
	client, testDomain := liveClient(t)
	safetyGate(t, client, testDomain)

	// Ensure domain exists for the alias tests
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

func TestLifecycleCredential_CreateUpdateDelete(t *testing.T) {
	skipIfNoLiveAPI(t)
	client, testDomain := liveClient(t)
	safetyGate(t, client, testDomain)

	// Ensure domain exists for the credential tests
	client.CreateDomain(testDomain, "")
	t.Cleanup(func() { client.DeleteDomain(testDomain) })

	integration.LifeCycleTest{
		Resource: "improvmx:index:SmtpCredential",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"domain":   property.New(testDomain),
				"username": property.New("lifecycle-sender"),
				"password": property.MakeSecret(property.New("T3stP@ss!")),
			}),
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"domain":   property.New(testDomain),
				"username": property.New("lifecycle-sender"),
				"password": property.MakeSecret(property.New("N3wP@ss!")),
			}),
		}},
	}.Run(t, lifecycleServer(t))
}
```

```go
func TestLifecycleDomain_CreateTwice_Idempotent(t *testing.T) {
	skipIfNoLiveAPI(t)
	client, testDomain := liveClient(t)
	safetyGate(t, client, testDomain)

	server := lifecycleServer(t)

	// First lifecycle: create and destroy
	integration.LifeCycleTest{
		Resource: "improvmx:index:Domain",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"domain": property.New(testDomain),
			}),
		},
	}.Run(t, server)

	// Second lifecycle: create again (should adopt the existing domain if not fully cleaned)
	integration.LifeCycleTest{
		Resource: "improvmx:index:Domain",
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"domain": property.New(testDomain),
			}),
		},
	}.Run(t, server)
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
				"password": property.MakeSecret(property.New("T3stP@ss!")),
			}),
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"domain":   property.New(testDomain),
				"username": property.New("new-sender"),
				"password": property.MakeSecret(property.New("T3stP@ss!")),
			}),
		}},
	}.Run(t, lifecycleServer(t))
}
```

Note: The `semver` package is `github.com/blang/semver` which is already a transitive dependency of `pulumi-go-provider`. If not, run `go get github.com/blang/semver`.

- [ ] **Step 2: Verify compilation**

Run: `go build ./provider/...`
Expected: No errors. If `blang/semver` is missing: `go get github.com/blang/semver`

- [ ] **Step 3: Commit**

```bash
git add provider/lifecycle_test.go
git commit -m "test: add Pulumi lifecycle tests for Domain, EmailAlias, and SmtpCredential"
```

---

## Chunk 4: Run and Verify (Task 9)

### Task 9: Run all tests and verify

- [ ] **Step 1: Run all unit tests (no live API needed)**

Run: `go test -v -count=1 -run 'Test(ListDomains|CreateDomain|GetDomain|DeleteDomain|CreateAlias|UpdateAlias|CreateSmtpCredential|APIError|BasicAuth|AlreadyExists|NotFound|AuthFailure|UpdateDomain|ListAliases|DeleteAlias|ListSmtpCredentials|UpdateSmtpCredential|DeleteSmtpCredential|DomainCreate|DomainRead|DomainUpdate|DomainDelete|DomainDiff|AliasCreate|AliasRead|AliasDelete|AliasDiff|CredentialCreate|CredentialRead|CredentialDelete|CredentialDiff)' ./provider/...`
Expected: All unit tests PASS (22 client + 24 provider = 46 total).

- [ ] **Step 2: Run live tests (requires .env.local)**

Run: `make test_live`
Expected: Safety gate prints warning, all live tests PASS, domain is cleaned up.

- [ ] **Step 3: Verify test count**

Run: `go test -v -count=1 ./provider/... 2>&1 | grep -c '^--- PASS'`
Expected: 46+ (unit tests only, live tests skipped).

- [ ] **Step 4: Final commit if any adjustments needed**

```bash
git add -A
git commit -m "test: finalize comprehensive test coverage"
```

- [ ] **Step 5: Push**

```bash
git push origin main
```
