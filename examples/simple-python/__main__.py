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
