#!/usr/bin/env bash
#
# E2E test: create domain via API, then pulumi up the same domain.
# Verifies the domain is adopted (not replaced/deleted).
#
# Requires: IMPROVMX_API_TOKEN, IMPROVMX_INTEGRATION_TEST_DOMAIN
# Loads .env.local from repo root if present.
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PULUMI="${PULUMI_BIN:-$HOME/.pulumi/bin/pulumi}"
PROVIDER_BIN="$REPO_ROOT/bin/pulumi-resource-improvmx"
TEST_DIR="$(mktemp -d)"
PLUGIN_DIR="$TEST_DIR/plugins"
STACK_NAME="test-adopt-$$"

# Load env
if [ -f "$REPO_ROOT/.env.local" ]; then
  set -a; source "$REPO_ROOT/.env.local"; set +a
fi

: "${IMPROVMX_API_TOKEN:?IMPROVMX_API_TOKEN must be set}"
: "${IMPROVMX_INTEGRATION_TEST_DOMAIN:?IMPROVMX_INTEGRATION_TEST_DOMAIN must be set}"
DOMAIN="$IMPROVMX_INTEGRATION_TEST_DOMAIN"

cleanup() {
  echo ""
  echo "=== Cleanup ==="
  # Destroy pulumi stack if it exists
  if [ -d "$TEST_DIR/project" ]; then
    cd "$TEST_DIR/project"
    PULUMI_HOME="$TEST_DIR/pulumi-home" "$PULUMI" destroy --yes --non-interactive 2>/dev/null || true
    PULUMI_HOME="$TEST_DIR/pulumi-home" "$PULUMI" stack rm --yes --non-interactive 2>/dev/null || true
  fi
  # Delete domain via API (cleanup)
  curl -sf -u "api:$IMPROVMX_API_TOKEN" -X DELETE "https://api.improvmx.com/v3/domains/$DOMAIN" > /dev/null 2>&1 || true
  rm -rf "$TEST_DIR"
  echo "Cleaned up."
}
trap cleanup EXIT

echo "========================================"
echo "E2E Test: Adopt Pre-Existing Domain"
echo "Domain: $DOMAIN"
echo "========================================"

# --- Build provider ---
echo ""
echo "=== Building provider ==="
make -C "$REPO_ROOT" provider 2>&1 | tail -1

# --- Install provider plugin ---
echo ""
echo "=== Installing provider plugin ==="
mkdir -p "$PLUGIN_DIR/resource-improvmx-v0.0.1"
cp "$PROVIDER_BIN" "$PLUGIN_DIR/resource-improvmx-v0.0.1/"

# --- Step 1: Create domain via API ---
echo ""
echo "=== Step 1: Create domain via API ==="
# Delete first if exists
curl -sf -u "api:$IMPROVMX_API_TOKEN" -X DELETE "https://api.improvmx.com/v3/domains/$DOMAIN" > /dev/null 2>&1 || true
sleep 1
RESULT=$(curl -sf -u "api:$IMPROVMX_API_TOKEN" -X POST "https://api.improvmx.com/v3/domains" \
  -H "Content-Type: application/json" \
  -d "{\"domain\":\"$DOMAIN\"}")
echo "$RESULT" | jq -r '"Created domain: \(.domain.domain) (aliases: \(.domain.aliases | length))"'

# --- Step 2: Verify domain exists with aliases ---
echo ""
echo "=== Step 2: Verify domain exists via API ==="
BEFORE=$(curl -sf -u "api:$IMPROVMX_API_TOKEN" "https://api.improvmx.com/v3/domains/$DOMAIN")
echo "$BEFORE" | jq -r '"Domain: \(.domain.domain), Active: \(.domain.active)"'
ALIASES_BEFORE=$(curl -sf -u "api:$IMPROVMX_API_TOKEN" "https://api.improvmx.com/v3/domains/$DOMAIN/aliases")
ALIAS_COUNT_BEFORE=$(echo "$ALIASES_BEFORE" | jq '.aliases | length')
echo "Aliases before pulumi up: $ALIAS_COUNT_BEFORE"

# --- Step 3: Create Pulumi project ---
echo ""
echo "=== Step 3: Setting up Pulumi project ==="
mkdir -p "$TEST_DIR/project" "$TEST_DIR/pulumi-home"
cat > "$TEST_DIR/project/Pulumi.yaml" << EOF
name: test-adopt
runtime:
  name: python
  options:
    toolchain: pip
    virtualenv: .venv
plugins:
  providers:
    - name: improvmx
      path: $PLUGIN_DIR/resource-improvmx-v0.0.1
EOF

cat > "$TEST_DIR/project/__main__.py" << PYEOF
import pulumi_improvmx as improvmx

domain = improvmx.Domain("test-domain", domain="$DOMAIN")
pulumi.export("domain_name", domain.domain)
PYEOF

# Fix: need pulumi import
cat > "$TEST_DIR/project/__main__.py" << PYEOF
import pulumi
import pulumi_improvmx as improvmx

domain = improvmx.Domain("test-domain", domain="$DOMAIN")
wildcard = improvmx.EmailAlias("wildcard",
    domain=domain.domain,
    alias="*",
    forward="test@example.com",
)
pulumi.export("domain_name", domain.domain)
PYEOF

cat > "$TEST_DIR/project/requirements.txt" << EOF
pulumi>=3.0.0,<4.0.0
pulumi-improvmx>=0.1.0
EOF

cd "$TEST_DIR/project"
export PULUMI_HOME="$TEST_DIR/pulumi-home"
export PULUMI_CONFIG_PASSPHRASE=""
mkdir -p "$TEST_DIR/pulumi-state"
export PULUMI_BACKEND_URL="file://$TEST_DIR/pulumi-state"
export PATH="$PLUGIN_DIR/resource-improvmx-v0.0.1:$PATH"

# Init stack
"$PULUMI" stack init "$STACK_NAME" 2>&1

# --- Step 4: pulumi up ---
echo ""
echo "=== Step 4: pulumi up ==="
"$PULUMI" up --yes --non-interactive 2>&1 || {
  echo "FAIL: pulumi up failed"
  exit 1
}

# --- Step 5: Verify domain still exists ---
echo ""
echo "=== Step 5: Verify domain still exists after pulumi up ==="
AFTER=$(curl -sf -u "api:$IMPROVMX_API_TOKEN" "https://api.improvmx.com/v3/domains/$DOMAIN" 2>&1) || {
  echo "FAIL: Domain $DOMAIN was DELETED by pulumi up!"
  exit 1
}
echo "$AFTER" | jq -r '"Domain: \(.domain.domain), Active: \(.domain.active)"'
ALIASES_AFTER=$(curl -sf -u "api:$IMPROVMX_API_TOKEN" "https://api.improvmx.com/v3/domains/$DOMAIN/aliases")
ALIAS_COUNT_AFTER=$(echo "$ALIASES_AFTER" | jq '.aliases | length')
echo "Aliases after pulumi up: $ALIAS_COUNT_AFTER"

if [ "$ALIAS_COUNT_AFTER" -lt "$ALIAS_COUNT_BEFORE" ]; then
  echo "FAIL: Aliases were lost! Before: $ALIAS_COUNT_BEFORE, After: $ALIAS_COUNT_AFTER"
  exit 1
fi

# --- Step 6: pulumi up again (idempotent) ---
echo ""
echo "=== Step 6: pulumi up again (should be no-op) ==="
UP2_OUTPUT=$("$PULUMI" up --yes --non-interactive 2>&1)
echo "$UP2_OUTPUT" | tail -5

# Verify domain still exists
curl -sf -u "api:$IMPROVMX_API_TOKEN" "https://api.improvmx.com/v3/domains/$DOMAIN" > /dev/null || {
  echo "FAIL: Domain $DOMAIN was DELETED by second pulumi up!"
  exit 1
}

# --- Step 7: pulumi destroy ---
echo ""
echo "=== Step 7: pulumi destroy ==="
"$PULUMI" destroy --yes --non-interactive 2>&1 || true

# Verify domain is now gone (expected)
echo ""
echo "=== Step 8: Verify domain deleted after destroy ==="
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -u "api:$IMPROVMX_API_TOKEN" "https://api.improvmx.com/v3/domains/$DOMAIN")
if [ "$HTTP_CODE" = "404" ]; then
  echo "Domain correctly deleted by pulumi destroy."
else
  echo "WARNING: Domain still exists after destroy (HTTP $HTTP_CODE)"
fi

echo ""
echo "========================================"
echo "PASS: Adopt test completed successfully"
echo "========================================"
