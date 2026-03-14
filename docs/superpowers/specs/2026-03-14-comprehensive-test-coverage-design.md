# Comprehensive Test Coverage Design

## Problem

The provider has deleted live domains multiple times due to insufficient test coverage. Root causes include `DeleteBeforeReplace` being unconditionally true, untested adopt-on-create flows, and no lifecycle tests verifying Diff semantics. The existing tests cover only basic HTTP client serialization and simple live CRUD — no error paths, no provider-level logic, no Pulumi lifecycle testing.

## Environment Variables

| Variable | Purpose |
|---|---|
| `IMPROVMX_API_TOKEN` | API token (used by provider and tests) |
| `IMPROVMX_INTEGRATION_TEST_DOMAIN` | Dedicated test-only domain |
| `IMPROVMX_LIVE_TEST=1` | Opt-in gate for live and lifecycle tests |

Loaded from `.env.local` (gitignored) via `make test_live`. Tests error out immediately if values are empty.

## Test Layers

### Layer 1: Client Unit Tests (`client_test.go`)

Mock HTTP server. Runs in CI. Tests request/response serialization and error detection.

**Existing tests to keep:** TestListDomains, TestCreateDomain, TestGetDomain, TestDeleteDomain, TestCreateAlias, TestUpdateAlias, TestCreateSmtpCredential, TestAPIError, TestBasicAuth.

**New tests:**
- `TestCreateDomain_AlreadyExists` — 400 with "already registered", verify `IsAlreadyExists()` true
- `TestCreateAlias_AlreadyExists` — 400 with "update the existing", verify `IsAlreadyExists()` true
- `TestGetDomain_NotFound` — 404, verify `IsNotFound()` true
- `TestGetAlias_NotFound` — 404, verify `IsNotFound()` true
- `TestDeleteDomain_NotFound` — 404, verify `IsNotFound()` true
- `TestAuthFailure_401` — verify hard auth error (not `APIError`), verify `errors.As(*APIError)` fails
- `TestAuthFailure_403` — same for 403
- `TestUpdateDomain` — verify PUT with correct fields
- `TestUpdateSmtpCredential` — verify PUT with password
- `TestListSmtpCredentials` — verify response parsing
- `TestDeleteAlias` — verify DELETE method and path
- `TestDeleteSmtpCredential` — verify DELETE method and path
- `TestListAliases` — verify response parsing

**Fix:** `newTestClient` must return a cleanup function or register `t.Cleanup(server.Close)` to avoid server lifecycle leaks.

### Layer 2: Provider Unit Tests (`provider_test.go`)

Mock HTTP server maintaining in-memory state. Runs in CI. Tests Pulumi resource CRUD methods directly.

**Context injection:** The resource CRUD methods call `getClient(ctx)` which uses `infer.GetConfig[ProviderConfig](ctx)`. To make these testable with a mock server, refactor `getClient()` to check for a `baseURL` override via an env var (e.g., `IMPROVMX_BASE_URL`). In tests, set this env var to the mock server's URL. This avoids needing a full Pulumi provider context for unit tests.

For Diff tests, the Diff methods do not call `getClient()` or use the context — `context.Background()` is sufficient.

**Test helper:** `newMockAPIServer()` returns an `httptest.Server` with in-memory state for domains, aliases, and credentials. Responds like the real API: returns "already registered" for duplicate domains, "update the existing" for duplicate aliases, 404 for missing resources. Returns a cleanup function via `t.Cleanup(server.Close)`.

**Domain tests:**
- `TestDomainCreate` — create succeeds, returns correct state
- `TestDomainCreate_AdoptExisting` — create returns "already registered", provider falls back to Get+Update, returns state with synced fields (notificationEmail, webhook)
- `TestDomainRead` — returns correct inputs and state
- `TestDomainRead_NotFound` — returns empty ID
- `TestDomainUpdate` — updates notificationEmail and webhook
- `TestDomainDelete` — deletes successfully
- `TestDomainDelete_AlreadyGone` — 404 on delete succeeds (idempotent)
- `TestDomainDiff_NoChange` — no diff when inputs match state
- `TestDomainDiff_UpdateOnly` — changing notificationEmail produces Update diff, `DeleteBeforeReplace` false
- `TestDomainDiff_Replace` — changing domain produces UpdateReplace diff, `DeleteBeforeReplace` true

**EmailAlias tests:**
- `TestAliasCreate` — create succeeds
- `TestAliasCreate_AdoptAndUpdate` — adopt existing, forward gets updated
- `TestAliasRead_NotFound` — returns empty ID
- `TestAliasDelete_AlreadyGone` — idempotent
- `TestAliasDiff_UpdateForward` — Update kind, no replace
- `TestAliasDiff_ReplaceAlias` — changing alias triggers replace
- `TestAliasDiff_ReplaceDomain` — changing domain triggers replace

**SmtpCredential tests:**
- `TestCredentialCreate` — create succeeds
- `TestCredentialCreate_AdoptExisting` — adopt, update password, list to find credential
- `TestCredentialRead_NotFound` — credential missing from list returns empty ID
- `TestCredentialRead_DomainGone` — domain 404 returns empty ID
- `TestCredentialDelete_AlreadyGone` — idempotent
- `TestCredentialDiff_UpdatePassword` — Update kind, no replace
- `TestCredentialDiff_ReplaceUsername` — changing username triggers replace

### Layer 3: Live Integration Tests (`live_test.go`)

Real API calls. Gated by `IMPROVMX_LIVE_TEST=1`. Single-runner sequential execution.

**Safety gate (runs once before any live test):**
1. Check `IMPROVMX_LIVE_TEST=1`
2. Print warning: `"WARNING: Live tests will CREATE and DELETE resources on domain %s. The domain itself WILL be destroyed."`
3. If domain exists on ImprovMX, verify zero aliases AND zero SMTP credentials. If either exist: `t.Fatal("Refusing to run: domain %s has existing resources. Use a clean test domain.")`

**Structure — single `TestLive` with ordered subtests:**

```
TestLive/
  Domain/
    Create
    Read
    Update_NotificationEmail
    Read_AfterUpdate
    CreateAgain_Adopt
    Delete
    Delete_AlreadyGone
    Read_AfterDelete
    Recreate
    Delete_Final
  EmailAlias/
    SetupDomain
    Create
    Read
    Update_Forward
    Read_AfterUpdate
    CreateAgain_Adopt
    Delete
    Delete_AlreadyGone
    Read_AfterDelete
    WildcardCreate
    WildcardDelete
    CleanupDomain
  SmtpCredential/
    SetupDomain
    Create
    Read
    Update_Password
    Read_AfterUpdate
    CreateAgain_Adopt
    Delete
    Delete_AlreadyGone
    Read_AfterDelete
    CleanupDomain
```

### Layer 4: Pulumi Lifecycle Tests (`lifecycle_test.go`)

Uses `pulumi-go-provider/integration` framework against the live API. Same safety gate as Layer 3.

**Tests:**
- `TestLifecycleDomain_CreateUpdateDelete` — up with domain, update notificationEmail, destroy
- `TestLifecycleDomain_CreateTwice_Idempotent` — up, destroy state only, up again (verify adopt)
- `TestLifecycleAlias_CreateUpdateDelete` — up with alias, update forward, destroy
- `TestLifecycleAlias_ReplaceOnAliasChange` — up, change alias name, verify replace
- `TestLifecycleCredential_CreateUpdateDelete` — up with credential, update password, destroy
- `TestLifecycleCredential_ReplaceOnUsernameChange` — up, change username, verify replace

## Other Changes

- Add `.env.local` to `.gitignore`
- Add `make test_live` target that loads `.env.local` and runs with `IMPROVMX_LIVE_TEST=1`. Replaces the old `test_integration` target.
- Delete old `integration_test.go` (replaced by `live_test.go`)
- Standardize on `IMPROVMX_INTEGRATION_TEST_DOMAIN` env var name in all tests (update CLAUDE.md and Makefile references)
- No rate limiting or retry tests — the ImprovMX API has not exhibited rate limiting in practice, and the client has no retry logic. If this becomes an issue, it can be addressed separately.
