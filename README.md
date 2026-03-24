# Pulumi ImprovMX Provider

[![CI](https://github.com/lokkju/pulumi-improvmx/actions/workflows/ci.yml/badge.svg)](https://github.com/lokkju/pulumi-improvmx/actions/workflows/ci.yml)
[![Release](https://github.com/lokkju/pulumi-improvmx/actions/workflows/release.yml/badge.svg)](https://github.com/lokkju/pulumi-improvmx/actions/workflows/release.yml)
[![PyPI](https://img.shields.io/pypi/v/pulumi-improvmx)](https://pypi.org/project/pulumi-improvmx/)
[![npm](https://img.shields.io/npm/v/pulumi-improvmx)](https://www.npmjs.com/package/pulumi-improvmx)
[![NuGet](https://img.shields.io/nuget/v/Lokkju.Improvmx)](https://www.nuget.org/packages/Lokkju.Improvmx)
[![Go Reference](https://pkg.go.dev/badge/github.com/lokkju/pulumi-improvmx/sdk/go/improvmx.svg)](https://pkg.go.dev/github.com/lokkju/pulumi-improvmx/sdk/go/improvmx)

A native Pulumi provider for managing [ImprovMX](https://improvmx.com/) email forwarding resources.

## Resources

- **Domain** — Register and manage domains for email forwarding
- **EmailAlias** — Create email aliases with forwarding rules (including catch-all `*`)
- **SmtpCredential** — Manage SMTP credentials for sending email

## Installation

### Python

```bash
pip install pulumi-improvmx
```

### TypeScript/JavaScript

```bash
npm install pulumi-improvmx
```

### Go

```bash
go get github.com/lokkju/pulumi-improvmx/sdk/go/improvmx
```

## Configuration

Set your ImprovMX API token:

```bash
pulumi config set improvmx:apiToken --secret sk_xxxxx
```

Or via environment variable:

```bash
export IMPROVMX_API_TOKEN=sk_xxxxx
```

## Example (Python)

```python
import pulumi
import pulumi_improvmx as improvmx

domain = improvmx.Domain("my-domain", domain="example.com")

wildcard = improvmx.EmailAlias(
    "wildcard",
    domain=domain.domain,
    alias="*",
    forward="me@gmail.com",
)

pulumi.export("domain_active", domain.active)
```

## Development

```bash
# Build the provider binary
make provider

# Run unit tests
make test

# Run live integration tests (requires API token + test domain)
IMPROVMX_API_TOKEN=sk_xxx IMPROVMX_TEST_DOMAIN=test.example.com make test_integration

# Generate SDKs
make codegen

# Lint
make lint
```

## Releasing

Releases are triggered by pushing a version tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This builds provider binaries (linux/darwin/windows, amd64/arm64), creates a GitHub Release, generates SDKs, and publishes to PyPI and npm.

### Trusted Publishing Setup

Both PyPI and npm use OIDC trusted publishing — no API tokens or secrets are stored in GitHub.

#### PyPI

1. Go to [pypi.org](https://pypi.org) → your account → Publishing → "Add a new pending publisher"
2. Configure:
   - **Package name:** `pulumi_improvmx`
   - **Owner:** `lokkju`
   - **Repository:** `pulumi-improvmx`
   - **Workflow:** `release.yml`
   - **Environment:** `pypi`
3. Ensure a `pypi` environment exists in GitHub repo settings (Settings → Environments)

#### npm

1. Publish the package once manually (npm requires the package to exist first):
   ```bash
   cd sdk/nodejs && npm install && npm publish --access public
   ```
2. Add the trusted publisher:
   ```bash
   npx npm@latest trust github pulumi-improvmx --file release.yml --repository lokkju/pulumi-improvmx --environment npm --yes
   ```
3. Ensure an `npm` environment exists in GitHub repo settings (Settings → Environments)

#### NuGet

1. Log into [nuget.org](https://nuget.org) → your profile → **Trusted Publishing**
2. Add a new trusted publishing policy:
   - **Repository Owner:** `lokkju`
   - **Repository:** `pulumi-improvmx`
   - **Workflow File:** `release.yml`
   - **Environment:** `nuget`
3. Add your nuget.org username (profile name, not email) as a GitHub secret:
   ```bash
   gh secret set NUGET_USER --repo lokkju/pulumi-improvmx
   ```
4. Ensure a `nuget` environment exists in GitHub repo settings (Settings → Environments)

After setup, all subsequent releases publish automatically via GitHub Actions with no tokens needed.

## License

[Polyform Shield 1.0.0](LICENSE)
