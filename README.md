# Pulumi ImprovMX Provider

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
npm install @pulumi/improvmx
```

### Go

```bash
go get github.com/lokkju/improvmx/sdk/go/improvmx
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

## License

[Polyform Shield 1.0.0](LICENSE)
