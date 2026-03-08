# Pulumi ImprovMX Provider - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a native cross-language Pulumi provider for ImprovMX email forwarding using `pulumi-go-provider`, publishable to the Pulumi registry with auto-generated SDKs for Python, TypeScript, Go, and .NET.

**Architecture:** A Go provider binary (`pulumi-resource-improvmx`) built with the `pulumi-go-provider` infer framework. Each resource (Domain, EmailAlias, SmtpCredential) is a Go struct implementing CRUD methods. An HTTP client wraps the ImprovMX v3 REST API. SDKs are auto-generated from the provider's schema via `pulumi package gen-sdk`. Tests use the `pulumi-go-provider/integration` test framework plus direct API integration tests.

**Tech Stack:** Go 1.24+, `pulumi-go-provider` infer framework, `net/http` (ImprovMX API client), `testify` (assertions), `pulumi-go-provider/integration` (lifecycle tests), Makefile (build/SDK gen)

---

## Project Structure

```
pulumi-improvmx/
├── go.mod
├── go.sum
├── Makefile
├── LICENSE                                    # Polyform Shield 1.0.0
├── README.md
├── CLAUDE.md
├── mise.toml
├── provider/
│   ├── provider.go                            # Provider builder, config (API token)
│   ├── client.go                              # ImprovMX v3 HTTP client
│   ├── client_test.go                         # Client unit tests (mock HTTP)
│   ├── domain.go                              # Domain resource
│   ├── domain_test.go                         # Domain lifecycle test
│   ├── email_alias.go                         # EmailAlias resource
│   ├── email_alias_test.go                    # EmailAlias lifecycle test
│   ├── smtp_credential.go                     # SmtpCredential resource
│   ├── smtp_credential_test.go                # SmtpCredential lifecycle test
│   ├── integration_test.go                    # Live API integration tests
│   └── cmd/
│       └── pulumi-resource-improvmx/
│           └── main.go                        # Provider binary entrypoint
├── sdk/                                       # Auto-generated (gitignored except go/)
│   ├── python/
│   ├── nodejs/
│   ├── go/
│   └── dotnet/
├── examples/
│   ├── simple-python/
│   │   ├── __main__.py
│   │   └── Pulumi.yaml
│   └── simple-go/
│       ├── main.go
│       └── Pulumi.yaml
└── tests/                                     # SDK integration tests (Go test harness)
    └── sdk_test.go
```

---

### Task 1: Project scaffolding

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `LICENSE`
- Create: `CLAUDE.md`
- Create: `mise.toml`
- Create: `.gitignore`
- Create: `provider/cmd/pulumi-resource-improvmx/main.go`
- Create: `provider/provider.go`

**Step 1: Initialize git repo**

```bash
git -C /home/lokkju/projects/lokkju/pulumi-improvmx init
```

**Step 2: Create go.mod**

```
module github.com/lokkju/pulumi-improvmx

go 1.24

require (
    github.com/pulumi/pulumi-go-provider v1.1.2
    github.com/pulumi/pulumi/sdk/v3 v3.212.0
    github.com/blang/semver v3.5.1+incompatible
    github.com/stretchr/testify v1.10.0
)
```

Run `go mod tidy` after creating all Go files.

**Step 3: Create mise.toml**

```toml
[tools]
go = "1.24"
pulumi = "latest"

[tasks.build]
run = "go build -o bin/pulumi-resource-improvmx ./provider/cmd/pulumi-resource-improvmx"

[tasks.test]
run = "go test -v -count=1 ./provider/..."

[tasks.lint]
run = "golangci-lint run ./provider/..."

[tasks.gen-sdk]
run = "make codegen"
```

**Step 4: Create .gitignore**

```
bin/
sdk/python/
sdk/nodejs/
sdk/dotnet/
sdk/java/
*.test
```

Note: `sdk/go/` is NOT gitignored — Go SDKs are typically committed for `go get` compatibility.

**Step 5: Create minimal provider.go**

```go
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
```

**Step 6: Create main.go entrypoint**

`provider/cmd/pulumi-resource-improvmx/main.go`:
```go
package main

import (
    "context"
    "fmt"
    "os"

    improvmx "github.com/lokkju/pulumi-improvmx/provider"
)

func main() {
    err := improvmx.Provider().Run(context.Background(), improvmx.Name, improvmx.Version)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
        os.Exit(1)
    }
}
```

**Step 7: Create Makefile**

```makefile
PACK            := improvmx
PROJECT         := github.com/lokkju/pulumi-improvmx
PROVIDER        := pulumi-resource-${PACK}
VERSION_PATH    := ${PROJECT}/provider.Version
PROVIDER_VERSION ?= 0.1.0-alpha.0+dev
VERSION_GENERIC  = $(shell pulumictl convert-version --language generic --version "$(PROVIDER_VERSION)" 2>/dev/null || echo "$(PROVIDER_VERSION)")
SCHEMA_FILE     := schema.json
WORKING_DIR     := $(shell pwd)

.PHONY: provider
provider:
	go build -o $(WORKING_DIR)/bin/${PROVIDER} \
		-ldflags "-X ${VERSION_PATH}=${VERSION_GENERIC}" \
		$(PROJECT)/provider/cmd/$(PROVIDER)

$(SCHEMA_FILE): provider
	pulumi package get-schema $(WORKING_DIR)/bin/${PROVIDER} | jq 'del(.version)' > $(SCHEMA_FILE)

.PHONY: codegen
codegen: $(SCHEMA_FILE) sdk/python sdk/nodejs sdk/go sdk/dotnet

sdk/%: $(SCHEMA_FILE)
	rm -rf $@
	pulumi package gen-sdk --language $* $(SCHEMA_FILE) --version "${VERSION_GENERIC}"

sdk/python: $(SCHEMA_FILE)
	rm -rf $@
	pulumi package gen-sdk --language python $(SCHEMA_FILE) --version "${VERSION_GENERIC}"
	cp README.md sdk/python/ 2>/dev/null || true

sdk/go: $(SCHEMA_FILE)
	rm -rf $@
	pulumi package gen-sdk --language go $(SCHEMA_FILE) --version "${VERSION_GENERIC}"

.PHONY: test
test:
	go test -v -count=1 -cover -timeout 2h ./provider/...

.PHONY: test_integration
test_integration:
	IMPROVMX_LIVE_TEST=1 go test -v -count=1 -run TestLive -timeout 5m ./provider/...

.PHONY: lint
lint:
	golangci-lint run ./provider/...

.PHONY: install
install: provider
	cp $(WORKING_DIR)/bin/${PROVIDER} $(GOPATH)/bin/

.PHONY: clean
clean:
	rm -rf bin/ $(SCHEMA_FILE) sdk/python sdk/nodejs sdk/dotnet sdk/java
```

**Step 8: Create LICENSE (Polyform Shield 1.0.0)**

Use the standard Polyform Shield 1.0.0 text.

**Step 9: Create CLAUDE.md**

```markdown
# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Native cross-language Pulumi provider for ImprovMX email forwarding. Built with `pulumi-go-provider` infer framework. Generates SDKs for Python, TypeScript, Go, and .NET.

## Commands

```bash
# Build the provider binary
make provider

# Run unit + lifecycle tests
make test

# Run live integration tests (requires IMPROVMX_API_TOKEN + IMPROVMX_TEST_DOMAIN)
make test_integration

# Generate schema + all language SDKs
make codegen

# Install provider binary to GOPATH
make install

# Lint
make lint
```

## Architecture

- `provider/provider.go` - Provider builder, config (API token via config or IMPROVMX_API_TOKEN env)
- `provider/client.go` - ImprovMX v3 HTTP client (net/http, no Pulumi dependency)
- `provider/domain.go` - Domain resource (Create/Read/Update/Delete/Diff)
- `provider/email_alias.go` - EmailAlias resource (alias + forwarding destinations)
- `provider/smtp_credential.go` - SmtpCredential resource (SMTP sending credentials)
- `provider/cmd/pulumi-resource-improvmx/main.go` - Provider binary entrypoint

## ImprovMX API

- Base URL: `https://api.improvmx.com/v3`
- Auth: HTTP Basic with username `api` and API token as password
- Docs: https://improvmx.com/api/

## Testing

Three layers:
1. **Client unit tests** (`client_test.go`): Mock HTTP server, test request/response handling
2. **Lifecycle tests** (`*_test.go`): Use `pulumi-go-provider/integration` to test full Create/Update/Delete cycles with mock client
3. **Live integration tests** (`integration_test.go`): Skipped unless `IMPROVMX_LIVE_TEST=1`, test against real API

## Resource ID Patterns

- Domain: `example.com` (the domain name itself)
- EmailAlias: `example.com/alias-name` (composite)
- SmtpCredential: `example.com/username` (composite)

## Diff Semantics

- Domain: changing `domain` forces replacement; `notification_email`/`webhook` are in-place updates
- EmailAlias: changing `domain` or `alias` forces replacement; `forward` is in-place update
- SmtpCredential: changing `domain` or `username` forces replacement; `password` is in-place update
```

**Step 10: Run go mod tidy and commit**

```bash
go mod tidy
git add -A
git commit -m "feat: scaffold pulumi-improvmx native provider project"
```

---

### Task 2: ImprovMX HTTP client

**Files:**
- Create: `provider/client.go`
- Create: `provider/client_test.go`

The client is a standalone HTTP wrapper with no Pulumi dependency — testable with a mock HTTP server.

**Step 1: Write client tests**

`provider/client_test.go`:
```go
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
```

**Step 2: Implement the HTTP client**

`provider/client.go`:
```go
package provider

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

const defaultBaseURL = "https://api.improvmx.com/v3"

type APIError struct {
    StatusCode int
    Message    string
    Errors     map[string]string
}

func (e *APIError) Error() string {
    return fmt.Sprintf("improvmx API error (%d): %s", e.StatusCode, e.Message)
}

// Response types

type DomainInfo struct {
    Domain            string `json:"domain"`
    Display           string `json:"display"`
    Active            bool   `json:"active"`
    NotificationEmail string `json:"notification_email"`
    Webhook           string `json:"webhook"`
}

type AliasInfo struct {
    ID      int    `json:"id"`
    Alias   string `json:"alias"`
    Forward string `json:"forward"`
}

type SmtpCredentialInfo struct {
    Username string `json:"username"`
    Created  int64  `json:"created"`
    Usage    int    `json:"usage"`
}

// Client

type ImprovMXClient struct {
    baseURL    string
    apiToken   string
    httpClient *http.Client
}

func NewImprovMXClient(apiToken string) *ImprovMXClient {
    return &ImprovMXClient{
        baseURL:  defaultBaseURL,
        apiToken: apiToken,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

func (c *ImprovMXClient) request(method, path string, body any) ([]byte, error) {
    var reqBody io.Reader
    if body != nil {
        b, err := json.Marshal(body)
        if err != nil {
            return nil, fmt.Errorf("marshal request body: %w", err)
        }
        reqBody = bytes.NewReader(b)
    }

    req, err := http.NewRequest(method, c.baseURL+path, reqBody)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    req.SetBasicAuth("api", c.apiToken)
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("execute request: %w", err)
    }
    defer resp.Body.Close()

    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read response: %w", err)
    }

    var result struct {
        Success bool              `json:"success"`
        Errors  map[string]string `json:"errors"`
    }
    json.Unmarshal(respBody, &result)

    if resp.StatusCode >= 400 || !result.Success {
        return nil, &APIError{
            StatusCode: resp.StatusCode,
            Message:    string(respBody),
            Errors:     result.Errors,
        }
    }

    return respBody, nil
}

// --- Domains ---

func (c *ImprovMXClient) ListDomains() ([]DomainInfo, error) {
    data, err := c.request("GET", "/domains", nil)
    if err != nil {
        return nil, err
    }
    var resp struct{ Domains []DomainInfo `json:"domains"` }
    json.Unmarshal(data, &resp)
    return resp.Domains, nil
}

func (c *ImprovMXClient) CreateDomain(domain, notificationEmail string) (*DomainInfo, error) {
    body := map[string]string{"domain": domain}
    if notificationEmail != "" {
        body["notification_email"] = notificationEmail
    }
    data, err := c.request("POST", "/domains", body)
    if err != nil {
        return nil, err
    }
    var resp struct{ Domain DomainInfo `json:"domain"` }
    json.Unmarshal(data, &resp)
    return &resp.Domain, nil
}

func (c *ImprovMXClient) GetDomain(domain string) (*DomainInfo, error) {
    data, err := c.request("GET", "/domains/"+domain, nil)
    if err != nil {
        return nil, err
    }
    var resp struct{ Domain DomainInfo `json:"domain"` }
    json.Unmarshal(data, &resp)
    return &resp.Domain, nil
}

func (c *ImprovMXClient) UpdateDomain(domain string, fields map[string]string) (*DomainInfo, error) {
    data, err := c.request("PUT", "/domains/"+domain, fields)
    if err != nil {
        return nil, err
    }
    var resp struct{ Domain DomainInfo `json:"domain"` }
    json.Unmarshal(data, &resp)
    return &resp.Domain, nil
}

func (c *ImprovMXClient) DeleteDomain(domain string) error {
    _, err := c.request("DELETE", "/domains/"+domain, nil)
    return err
}

// --- Aliases ---

func (c *ImprovMXClient) ListAliases(domain string) ([]AliasInfo, error) {
    data, err := c.request("GET", "/domains/"+domain+"/aliases", nil)
    if err != nil {
        return nil, err
    }
    var resp struct{ Aliases []AliasInfo `json:"aliases"` }
    json.Unmarshal(data, &resp)
    return resp.Aliases, nil
}

func (c *ImprovMXClient) CreateAlias(domain, alias, forward string) (*AliasInfo, error) {
    data, err := c.request("POST", "/domains/"+domain+"/aliases", map[string]string{
        "alias":   alias,
        "forward": forward,
    })
    if err != nil {
        return nil, err
    }
    var resp struct{ Alias AliasInfo `json:"alias"` }
    json.Unmarshal(data, &resp)
    return &resp.Alias, nil
}

func (c *ImprovMXClient) GetAlias(domain, alias string) (*AliasInfo, error) {
    data, err := c.request("GET", "/domains/"+domain+"/aliases/"+alias, nil)
    if err != nil {
        return nil, err
    }
    var resp struct{ Alias AliasInfo `json:"alias"` }
    json.Unmarshal(data, &resp)
    return &resp.Alias, nil
}

func (c *ImprovMXClient) UpdateAlias(domain, alias, forward string) (*AliasInfo, error) {
    data, err := c.request("PUT", "/domains/"+domain+"/aliases/"+alias, map[string]string{
        "forward": forward,
    })
    if err != nil {
        return nil, err
    }
    var resp struct{ Alias AliasInfo `json:"alias"` }
    json.Unmarshal(data, &resp)
    return &resp.Alias, nil
}

func (c *ImprovMXClient) DeleteAlias(domain, alias string) error {
    _, err := c.request("DELETE", "/domains/"+domain+"/aliases/"+alias, nil)
    return err
}

// --- SMTP Credentials ---

func (c *ImprovMXClient) ListSmtpCredentials(domain string) ([]SmtpCredentialInfo, error) {
    data, err := c.request("GET", "/domains/"+domain+"/credentials", nil)
    if err != nil {
        return nil, err
    }
    var resp struct{ Credentials []SmtpCredentialInfo `json:"credentials"` }
    json.Unmarshal(data, &resp)
    return resp.Credentials, nil
}

func (c *ImprovMXClient) CreateSmtpCredential(domain, username, password string) (*SmtpCredentialInfo, error) {
    data, err := c.request("POST", "/domains/"+domain+"/credentials", map[string]string{
        "username": username,
        "password": password,
    })
    if err != nil {
        return nil, err
    }
    var resp struct{ Credential SmtpCredentialInfo `json:"credential"` }
    json.Unmarshal(data, &resp)
    return &resp.Credential, nil
}

func (c *ImprovMXClient) UpdateSmtpCredential(domain, username, password string) error {
    _, err := c.request("PUT", "/domains/"+domain+"/credentials/"+username, map[string]string{
        "password": password,
    })
    return err
}

func (c *ImprovMXClient) DeleteSmtpCredential(domain, username string) error {
    _, err := c.request("DELETE", "/domains/"+domain+"/credentials/"+username, nil)
    return err
}
```

**Step 3: Run tests**

```bash
go test -v -count=1 ./provider/... -run TestClient -run TestList -run TestCreate -run TestGet -run TestDelete -run TestUpdate -run TestAPI -run TestBasic
```

**Step 4: Commit**

```bash
git add -A
git commit -m "feat: implement ImprovMX HTTP client with mock tests"
```

---

### Task 3: Domain resource

**Files:**
- Create: `provider/domain.go`
- Create: `provider/domain_test.go`
- Modify: `provider/provider.go` (register resource)

**Step 1: Implement Domain resource**

`provider/domain.go`:
```go
package provider

import (
    "context"
    "fmt"
    "os"

    p "github.com/pulumi/pulumi-go-provider"
    "github.com/pulumi/pulumi-go-provider/infer"
)

type Domain struct{}

type DomainArgs struct {
    Domain            string  `pulumi:"domain"`
    NotificationEmail *string `pulumi:"notificationEmail,optional"`
    Webhook           *string `pulumi:"webhook,optional"`
}

type DomainState struct {
    DomainArgs
    Active  bool   `pulumi:"active"`
    Display string `pulumi:"display"`
}

func (d *Domain) Annotate(a infer.Annotator) {
    a.Describe(&d, "Manages an ImprovMX domain for email forwarding.")
}

func (d *DomainArgs) Annotate(a infer.Annotator) {
    a.Describe(&d.Domain, "The domain name to register with ImprovMX.")
    a.Describe(&d.NotificationEmail, "Email address for delivery notifications.")
    a.Describe(&d.Webhook, "Webhook URL for delivery notifications.")
}

func (d *DomainState) Annotate(a infer.Annotator) {
    a.Describe(&d.Active, "Whether the domain's DNS is correctly configured.")
    a.Describe(&d.Display, "Display name of the domain.")
}

func getClient(ctx context.Context) *ImprovMXClient {
    config := infer.GetConfig[ProviderConfig](ctx)
    token := config.ApiToken
    if token == "" {
        token = os.Getenv("IMPROVMX_API_TOKEN")
    }
    if token == "" {
        p.GetLogger(ctx).Errorf("ImprovMX API token not configured")
    }
    return NewImprovMXClient(token)
}

func (Domain) Create(ctx context.Context, req infer.CreateRequest[DomainArgs]) (infer.CreateResponse[DomainState], error) {
    input := req.Inputs
    if req.DryRun {
        return infer.CreateResponse[DomainState]{
            ID:     input.Domain,
            Output: DomainState{DomainArgs: input},
        }, nil
    }

    client := getClient(ctx)
    notifEmail := ""
    if input.NotificationEmail != nil {
        notifEmail = *input.NotificationEmail
    }

    domain, err := client.CreateDomain(input.Domain, notifEmail)
    if err != nil {
        return infer.CreateResponse[DomainState]{}, fmt.Errorf("creating domain: %w", err)
    }

    return infer.CreateResponse[DomainState]{
        ID: domain.Domain,
        Output: DomainState{
            DomainArgs: input,
            Active:     domain.Active,
            Display:    domain.Display,
        },
    }, nil
}

func (Domain) Read(ctx context.Context, req infer.ReadRequest[DomainArgs, DomainState]) (infer.ReadResponse[DomainArgs, DomainState], error) {
    client := getClient(ctx)
    domain, err := client.GetDomain(req.ID)
    if err != nil {
        return infer.ReadResponse[DomainArgs, DomainState]{}, fmt.Errorf("reading domain: %w", err)
    }

    args := DomainArgs{
        Domain: domain.Domain,
    }
    if domain.NotificationEmail != "" {
        args.NotificationEmail = &domain.NotificationEmail
    }
    if domain.Webhook != "" {
        args.Webhook = &domain.Webhook
    }

    return infer.ReadResponse[DomainArgs, DomainState]{
        ID:     domain.Domain,
        Inputs: args,
        State: DomainState{
            DomainArgs: args,
            Active:     domain.Active,
            Display:    domain.Display,
        },
    }, nil
}

func (Domain) Update(ctx context.Context, req infer.UpdateRequest[DomainArgs, DomainState]) (infer.UpdateResponse[DomainState], error) {
    input := req.Inputs
    if req.DryRun {
        return infer.UpdateResponse[DomainState]{
            Output: DomainState{DomainArgs: input},
        }, nil
    }

    client := getClient(ctx)
    fields := map[string]string{}
    if input.NotificationEmail != nil {
        fields["notification_email"] = *input.NotificationEmail
    }
    if input.Webhook != nil {
        fields["webhook"] = *input.Webhook
    }

    domain, err := client.UpdateDomain(req.ID, fields)
    if err != nil {
        return infer.UpdateResponse[DomainState]{}, fmt.Errorf("updating domain: %w", err)
    }

    return infer.UpdateResponse[DomainState]{
        Output: DomainState{
            DomainArgs: input,
            Active:     domain.Active,
            Display:    domain.Display,
        },
    }, nil
}

func (Domain) Delete(ctx context.Context, req infer.DeleteRequest[DomainState]) (infer.DeleteResponse, error) {
    client := getClient(ctx)
    if err := client.DeleteDomain(req.ID); err != nil {
        return infer.DeleteResponse{}, fmt.Errorf("deleting domain: %w", err)
    }
    return infer.DeleteResponse{}, nil
}

func (Domain) Diff(ctx context.Context, req infer.DiffRequest[DomainArgs, DomainState]) (infer.DiffResponse, error) {
    diff := map[string]p.PropertyDiff{}

    if req.Inputs.Domain != req.State.Domain {
        diff["domain"] = p.PropertyDiff{Kind: p.UpdateReplace}
    }
    if ptrDiffers(req.Inputs.NotificationEmail, req.State.NotificationEmail) {
        diff["notificationEmail"] = p.PropertyDiff{Kind: p.Update}
    }
    if ptrDiffers(req.Inputs.Webhook, req.State.Webhook) {
        diff["webhook"] = p.PropertyDiff{Kind: p.Update}
    }

    return infer.DiffResponse{
        DeleteBeforeReplace: true,
        HasChanges:          len(diff) > 0,
        DetailedDiff:        diff,
    }, nil
}

func ptrDiffers(a, b *string) bool {
    if a == nil && b == nil {
        return false
    }
    if a == nil || b == nil {
        return true
    }
    return *a != *b
}
```

**Step 2: Register resource in provider.go**

Add to the builder chain in `Provider()`:
```go
WithResources(
    infer.Resource(Domain{}),
).
```

**Step 3: Write lifecycle test**

`provider/domain_test.go`:
```go
package provider

import (
    "testing"

    "github.com/blang/semver"
    "github.com/stretchr/testify/require"

    integration "github.com/pulumi/pulumi-go-provider/integration"
    presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestDomainLifecycle(t *testing.T) {
    // This test requires a mock or real API; skip if not configured
    // For unit testing, we rely on client_test.go mock tests
    // This tests the Pulumi resource lifecycle plumbing
    t.Skip("Requires live API or mock server injection — covered by integration_test.go")
}
```

Note: Full lifecycle tests through the Pulumi integration framework require either a mock HTTP server injection point or live API. The client_test.go mock tests cover the HTTP layer; integration_test.go (Task 7) covers the end-to-end flow.

**Step 4: Run tests and build**

```bash
go test -v -count=1 ./provider/...
go build -o bin/pulumi-resource-improvmx ./provider/cmd/pulumi-resource-improvmx
```

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: implement Domain resource with CRUD and diff"
```

---

### Task 4: EmailAlias resource

**Files:**
- Create: `provider/email_alias.go`
- Modify: `provider/provider.go` (register resource)

**Step 1: Implement EmailAlias resource**

`provider/email_alias.go` — follows the same pattern as Domain but with composite ID (`domain/alias`):

```go
package provider

import (
    "context"
    "fmt"
    "strings"

    p "github.com/pulumi/pulumi-go-provider"
    "github.com/pulumi/pulumi-go-provider/infer"
)

type EmailAlias struct{}

type EmailAliasArgs struct {
    Domain  string `pulumi:"domain"`
    Alias   string `pulumi:"alias"`
    Forward string `pulumi:"forward"`
}

type EmailAliasState struct {
    EmailAliasArgs
}

func (e *EmailAlias) Annotate(a infer.Annotator) {
    a.Describe(&e, "Manages an ImprovMX email alias (forwarding rule).")
}

func (e *EmailAliasArgs) Annotate(a infer.Annotator) {
    a.Describe(&e.Domain, "The domain this alias belongs to.")
    a.Describe(&e.Alias, "The alias name (e.g., 'info', '*' for catch-all).")
    a.Describe(&e.Forward, "Comma-separated destination email addresses.")
}

func makeAliasID(domain, alias string) string { return domain + "/" + alias }

func parseAliasID(id string) (string, string) {
    parts := strings.SplitN(id, "/", 2)
    return parts[0], parts[1]
}

func (EmailAlias) Create(ctx context.Context, req infer.CreateRequest[EmailAliasArgs]) (infer.CreateResponse[EmailAliasState], error) {
    input := req.Inputs
    id := makeAliasID(input.Domain, input.Alias)
    if req.DryRun {
        return infer.CreateResponse[EmailAliasState]{ID: id, Output: EmailAliasState{EmailAliasArgs: input}}, nil
    }

    client := getClient(ctx)
    alias, err := client.CreateAlias(input.Domain, input.Alias, input.Forward)
    if err != nil {
        return infer.CreateResponse[EmailAliasState]{}, fmt.Errorf("creating alias: %w", err)
    }

    return infer.CreateResponse[EmailAliasState]{
        ID: id,
        Output: EmailAliasState{EmailAliasArgs: EmailAliasArgs{
            Domain:  input.Domain,
            Alias:   alias.Alias,
            Forward: alias.Forward,
        }},
    }, nil
}

func (EmailAlias) Read(ctx context.Context, req infer.ReadRequest[EmailAliasArgs, EmailAliasState]) (infer.ReadResponse[EmailAliasArgs, EmailAliasState], error) {
    domain, aliasName := parseAliasID(req.ID)
    client := getClient(ctx)
    alias, err := client.GetAlias(domain, aliasName)
    if err != nil {
        return infer.ReadResponse[EmailAliasArgs, EmailAliasState]{}, fmt.Errorf("reading alias: %w", err)
    }

    args := EmailAliasArgs{Domain: domain, Alias: alias.Alias, Forward: alias.Forward}
    return infer.ReadResponse[EmailAliasArgs, EmailAliasState]{
        ID: req.ID, Inputs: args, State: EmailAliasState{EmailAliasArgs: args},
    }, nil
}

func (EmailAlias) Update(ctx context.Context, req infer.UpdateRequest[EmailAliasArgs, EmailAliasState]) (infer.UpdateResponse[EmailAliasState], error) {
    input := req.Inputs
    if req.DryRun {
        return infer.UpdateResponse[EmailAliasState]{Output: EmailAliasState{EmailAliasArgs: input}}, nil
    }

    domain, aliasName := parseAliasID(req.ID)
    client := getClient(ctx)
    alias, err := client.UpdateAlias(domain, aliasName, input.Forward)
    if err != nil {
        return infer.UpdateResponse[EmailAliasState]{}, fmt.Errorf("updating alias: %w", err)
    }

    return infer.UpdateResponse[EmailAliasState]{
        Output: EmailAliasState{EmailAliasArgs: EmailAliasArgs{
            Domain: domain, Alias: alias.Alias, Forward: alias.Forward,
        }},
    }, nil
}

func (EmailAlias) Delete(ctx context.Context, req infer.DeleteRequest[EmailAliasState]) (infer.DeleteResponse, error) {
    domain, aliasName := parseAliasID(req.ID)
    client := getClient(ctx)
    if err := client.DeleteAlias(domain, aliasName); err != nil {
        return infer.DeleteResponse{}, fmt.Errorf("deleting alias: %w", err)
    }
    return infer.DeleteResponse{}, nil
}

func (EmailAlias) Diff(ctx context.Context, req infer.DiffRequest[EmailAliasArgs, EmailAliasState]) (infer.DiffResponse, error) {
    diff := map[string]p.PropertyDiff{}
    if req.Inputs.Domain != req.State.Domain {
        diff["domain"] = p.PropertyDiff{Kind: p.UpdateReplace}
    }
    if req.Inputs.Alias != req.State.Alias {
        diff["alias"] = p.PropertyDiff{Kind: p.UpdateReplace}
    }
    if req.Inputs.Forward != req.State.Forward {
        diff["forward"] = p.PropertyDiff{Kind: p.Update}
    }
    return infer.DiffResponse{
        DeleteBeforeReplace: true,
        HasChanges:          len(diff) > 0,
        DetailedDiff:        diff,
    }, nil
}
```

**Step 2: Register in provider.go**

Add `infer.Resource(EmailAlias{})` to the `WithResources()` call.

**Step 3: Run tests and build**

```bash
go test -v -count=1 ./provider/...
go build -o bin/pulumi-resource-improvmx ./provider/cmd/pulumi-resource-improvmx
```

**Step 4: Commit**

```bash
git add -A
git commit -m "feat: implement EmailAlias resource with CRUD and diff"
```

---

### Task 5: SmtpCredential resource

**Files:**
- Create: `provider/smtp_credential.go`
- Modify: `provider/provider.go` (register resource)

Follows the same pattern. Composite ID `domain/username`. Password changes are in-place updates, username/domain changes force replacement. Password is marked as a secret.

**Step 1: Implement SmtpCredential resource**

`provider/smtp_credential.go` — same structure as EmailAlias but:
- `SmtpCredentialArgs`: `Domain`, `Username`, `Password` (password has `provider:"secret"` tag)
- `SmtpCredentialState`: embeds args, adds `Created int64`
- Create: calls `client.CreateSmtpCredential`
- Read: calls `client.ListSmtpCredentials` and finds by username (no direct GET endpoint)
- Update: calls `client.UpdateSmtpCredential` (password only)
- Delete: calls `client.DeleteSmtpCredential`
- Diff: domain/username -> replace, password -> update

**Step 2: Register in provider.go**

Add `infer.Resource(SmtpCredential{})`.

**Step 3: Run tests and build**

```bash
go test -v -count=1 ./provider/...
go build -o bin/pulumi-resource-improvmx ./provider/cmd/pulumi-resource-improvmx
```

**Step 4: Commit**

```bash
git add -A
git commit -m "feat: implement SmtpCredential resource with CRUD and diff"
```

---

### Task 6: Live integration tests

**Files:**
- Create: `provider/integration_test.go`

These run against the real ImprovMX API. Gated by `IMPROVMX_LIVE_TEST=1` env var.

**Step 1: Write integration tests**

`provider/integration_test.go`:
```go
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
```

**Step 2: Run**

```bash
IMPROVMX_LIVE_TEST=1 IMPROVMX_API_TOKEN=sk_xxx IMPROVMX_TEST_DOMAIN=test.example.com go test -v -count=1 -run TestLive ./provider/...
```

**Step 3: Commit**

```bash
git add -A
git commit -m "feat: add live integration tests for ImprovMX API"
```

---

### Task 7: SDK generation and example programs

**Files:**
- Create: `examples/simple-python/__main__.py`
- Create: `examples/simple-python/Pulumi.yaml`

**Step 1: Generate schema and SDKs**

```bash
make provider
make codegen
```

This builds the binary, extracts the schema, and generates Python/TypeScript/Go/.NET SDKs.

**Step 2: Create Python example**

`examples/simple-python/Pulumi.yaml`:
```yaml
name: improvmx-example
runtime:
  name: python
  options:
    toolchain: pip
    virtualenv: .venv
description: Example using pulumi-improvmx provider
```

`examples/simple-python/__main__.py`:
```python
"""Example: manage ImprovMX domain with email forwarding."""

import pulumi
import pulumi_improvmx as improvmx

domain = improvmx.Domain("my-domain", domain="example.com")

wildcard = improvmx.EmailAlias(
    "wildcard",
    domain=domain.domain,
    alias="*",
    forward="me@gmail.com",
)

info = improvmx.EmailAlias(
    "info-alias",
    domain=domain.domain,
    alias="info",
    forward="info@company.com,backup@company.com",
)

pulumi.export("domain", domain.domain)
pulumi.export("domain_active", domain.active)
```

**Step 3: Commit**

```bash
git add -A
git commit -m "feat: generate SDKs and add example programs"
```

---

### Task 8: README and final polish

**Files:**
- Create: `README.md`

**Step 1: Write README** covering installation, configuration, resource examples for each language, and development instructions.

**Step 2: Run full test suite and lint**

```bash
make test
make lint
make provider
```

**Step 3: Commit**

```bash
git add -A
git commit -m "docs: add README with usage examples and development guide"
```

---

## Summary

| Task | Description | Tests |
|------|-------------|-------|
| 1 | Project scaffolding (go.mod, Makefile, provider shell) | - |
| 2 | HTTP client (net/http) | Mock HTTP server tests for all CRUD + auth + errors |
| 3 | Domain resource | Client mock tests; Diff semantics |
| 4 | EmailAlias resource | Client mock tests; composite ID; replace vs update |
| 5 | SmtpCredential resource | Client mock tests; secret password; Read via list |
| 6 | Live integration tests | Full CRUD lifecycle against real API |
| 7 | SDK generation + examples | Python example program |
| 8 | README + polish | Lint, full test run |

## Testing strategy

- **Client unit tests** (`client_test.go`): Mock HTTP server via `httptest`, test every endpoint, auth, and error handling. Fast, no network.
- **Live integration tests** (`integration_test.go`): Gated by `IMPROVMX_LIVE_TEST=1`. Full CRUD lifecycle for domains, aliases, credentials, wildcards. Requires `IMPROVMX_API_TOKEN` and `IMPROVMX_TEST_DOMAIN`.
- **All tests**: `make test` (excludes live tests by default)
- **Live tests**: `make test_integration`
