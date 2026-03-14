package provider

import (
	"context"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMockEnv(t *testing.T, baseURL string) {
	t.Helper()
	t.Setenv("IMPROVMX_BASE_URL", baseURL)
	t.Setenv("IMPROVMX_API_TOKEN", "mock-token")
}

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
	state.mu.Lock()
	state.domains["test.com"] = &mockDomain{Domain: "test.com", Display: "test.com", Active: false}
	state.aliases["test.com"] = make(map[string]*mockAlias)
	state.credentials["test.com"] = make(map[string]*mockCredential)
	state.mu.Unlock()
	email := "admin@test.com"
	webhook := "https://hooks.test.com/notify"
	resp, err := Domain{}.Create(context.Background(), infer.CreateRequest[DomainArgs]{
		Inputs: DomainArgs{Domain: "test.com", NotificationEmail: &email, Webhook: &webhook},
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
		ID: "test.com", Inputs: DomainArgs{Domain: "test.com", NotificationEmail: &email},
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
	assert.False(t, resp.DeleteBeforeReplace, "should never delete-before-replace to avoid cascade deletes")
	assert.Equal(t, p.UpdateReplace, resp.DetailedDiff["domain"].Kind)
}

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
	state.aliases["test.com"] = map[string]*mockAlias{"info": {ID: 1, Alias: "info", Forward: "old@gmail.com"}}
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
		ID: "test.com/gone", State: EmailAliasState{EmailAliasArgs: EmailAliasArgs{Domain: "test.com", Alias: "gone"}},
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
	assert.False(t, resp.DeleteBeforeReplace, "should never delete-before-replace")
	assert.Equal(t, p.UpdateReplace, resp.DetailedDiff["alias"].Kind)
}

func TestAliasDiff_ReplaceDomain(t *testing.T) {
	resp, err := EmailAlias{}.Diff(context.Background(), infer.DiffRequest[EmailAliasArgs, EmailAliasState]{
		Inputs: EmailAliasArgs{Domain: "new.com", Alias: "info", Forward: "user@gmail.com"},
		State:  EmailAliasState{EmailAliasArgs: EmailAliasArgs{Domain: "old.com", Alias: "info", Forward: "user@gmail.com"}},
	})
	require.NoError(t, err)
	assert.True(t, resp.HasChanges)
	assert.False(t, resp.DeleteBeforeReplace, "should never delete-before-replace")
	assert.Equal(t, p.UpdateReplace, resp.DetailedDiff["domain"].Kind)
}

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
	state.credentials["test.com"] = map[string]*mockCredential{"sender": {Username: "sender", Created: 1700000000}}
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
		ID: "test.com/gone", State: SmtpCredentialState{SmtpCredentialArgs: SmtpCredentialArgs{Password: "old"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "", resp.ID)
}

func TestCredentialRead_DomainGone(t *testing.T) {
	server, _ := newMockAPIServer(t)
	setupMockEnv(t, server.URL)
	resp, err := SmtpCredential{}.Read(context.Background(), infer.ReadRequest[SmtpCredentialArgs, SmtpCredentialState]{
		ID: "gone.com/sender", State: SmtpCredentialState{SmtpCredentialArgs: SmtpCredentialArgs{Password: "old"}},
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
		ID: "test.com/gone", State: SmtpCredentialState{SmtpCredentialArgs: SmtpCredentialArgs{Domain: "test.com", Username: "gone"}},
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
	assert.False(t, resp.DeleteBeforeReplace, "should never delete-before-replace")
	assert.Equal(t, p.UpdateReplace, resp.DetailedDiff["username"].Kind)
}
