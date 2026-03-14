# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Native cross-language Pulumi provider for [ImprovMX](https://improvmx.com/) email forwarding. Built with Go using the `pulumi-go-provider` infer framework. Auto-generates SDKs for Python, TypeScript, Go, and .NET.

## Build & Development Commands

```bash
make provider              # Build the provider binary
make test                  # Unit + lifecycle tests
make test_live             # Live integration tests (loads .env.local, requires IMPROVMX_LIVE_TEST=1)
make codegen               # Generate SDKs (Python, Node.js, Go, .NET)
make install               # Install provider to GOPATH
make lint                  # Run golangci-lint
```

Run a single test:
```bash
go test ./provider/ -run TestDomainLifecycle -v
```

**Environment:** Go 1.24+, Pulumi CLI, golangci-lint. Use `mise install` to set up tools via `mise.toml`.

## Architecture

**Provider binary:** `provider/cmd/pulumi-resource-improvmx/main.go` — entrypoint that wires up the provider.

**Provider config:** `provider/provider.go` — defines provider configuration (API token via config or `IMPROVMX_API_TOKEN` env var).

**HTTP client:** `provider/client.go` — wraps the ImprovMX v3 REST API (`https://api.improvmx.com/v3`). Auth is HTTP Basic with username `api` and API token as password.

### Resources

Each resource is a Go struct implementing CRUD methods via the `pulumi-go-provider` infer framework:

| Resource | File | ID Pattern | Replace on change |
|---|---|---|---|
| **Domain** | `provider/domain.go` | domain name (`example.com`) | `domain` |
| **EmailAlias** | `provider/email_alias.go` | composite (`example.com/alias`) | `domain`, `alias` |
| **SmtpCredential** | `provider/smtp_credential.go` | composite (`example.com/username`) | `domain`, `username` |

### Testing Layers

1. **Client unit tests** (`provider/client_test.go`) — mock HTTP server, verify request/response serialization
2. **Provider unit tests** (`provider/provider_test.go`) — CRUD + Diff with mock API
3. **Live integration tests** (`provider/live_test.go`) — real API calls, gated by `IMPROVMX_LIVE_TEST=1` env var; requires `IMPROVMX_API_TOKEN` and `IMPROVMX_INTEGRATION_TEST_DOMAIN`
4. **Pulumi lifecycle tests** (`provider/lifecycle_test.go`) — uses `pulumi-go-provider/integration`

### SDK Generation

SDKs in `sdk/` are auto-generated via `pulumi package gen-sdk`. All SDK directories except `sdk/go/` are gitignored.

## Implementation Plan

Detailed implementation plan with code samples: `docs/plans/2026-03-07-pulumi-improvmx-provider.md`
