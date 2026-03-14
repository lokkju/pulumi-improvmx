package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// --- State types ---

type mockDomain struct {
	Domain            string `json:"domain"`
	Display           string `json:"display"`
	Active            bool   `json:"active"`
	NotificationEmail string `json:"notification_email"`
	Webhook           string `json:"webhook"`
}

type mockAlias struct {
	ID      int    `json:"id"`
	Alias   string `json:"alias"`
	Forward string `json:"forward"`
}

type mockCredential struct {
	Username string `json:"username"`
	Created  int64  `json:"created"`
	Usage    int    `json:"usage"`
}

type mockAPIState struct {
	mu          sync.Mutex
	domains     map[string]*mockDomain
	aliases     map[string]map[string]*mockAlias // domain -> alias -> mockAlias
	credentials map[string]map[string]*mockCredential
	nextAliasID int
}

// --- Server constructor ---

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
		var body struct {
			Domain            string `json:"domain"`
			NotificationEmail string `json:"notification_email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Domain == "" {
			jsonResponse(w, 400, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "domain is required"},
			})
			return
		}
		state.mu.Lock()
		defer state.mu.Unlock()
		if _, exists := state.domains[body.Domain]; exists {
			jsonResponse(w, 400, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "Domain already registered on this account."},
			})
			return
		}
		d := &mockDomain{
			Domain:            body.Domain,
			Display:           body.Domain,
			Active:            false,
			NotificationEmail: body.NotificationEmail,
		}
		state.domains[body.Domain] = d
		state.aliases[body.Domain] = make(map[string]*mockAlias)
		state.credentials[body.Domain] = make(map[string]*mockCredential)
		jsonResponse(w, 200, map[string]any{
			"success": true,
			"domain":  d,
		})
	})

	// GET /domains/{domain}
	mux.HandleFunc("GET /domains/{domain}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("domain")
		state.mu.Lock()
		defer state.mu.Unlock()
		d, ok := state.domains[name]
		if !ok {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "not found"},
			})
			return
		}
		jsonResponse(w, 200, map[string]any{
			"success": true,
			"domain":  d,
		})
	})

	// PUT /domains/{domain}
	mux.HandleFunc("PUT /domains/{domain}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("domain")
		var fields map[string]string
		json.NewDecoder(r.Body).Decode(&fields)
		state.mu.Lock()
		defer state.mu.Unlock()
		d, ok := state.domains[name]
		if !ok {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "not found"},
			})
			return
		}
		if v, ok := fields["notification_email"]; ok {
			d.NotificationEmail = v
		}
		if v, ok := fields["webhook"]; ok {
			d.Webhook = v
		}
		jsonResponse(w, 200, map[string]any{
			"success": true,
			"domain":  d,
		})
	})

	// GET /domains/{domain}/check
	mux.HandleFunc("GET /domains/{domain}/check", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]any{
			"success": true,
			"records": map[string]any{"valid": true},
		})
	})

	// DELETE /domains/{domain}
	mux.HandleFunc("DELETE /domains/{domain}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("domain")
		state.mu.Lock()
		defer state.mu.Unlock()
		if _, ok := state.domains[name]; !ok {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "not found"},
			})
			return
		}
		delete(state.domains, name)
		delete(state.aliases, name)
		delete(state.credentials, name)
		jsonResponse(w, 200, map[string]any{"success": true})
	})

	// POST /domains/{domain}/aliases
	mux.HandleFunc("POST /domains/{domain}/aliases", func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")
		var body struct {
			Alias   string `json:"alias"`
			Forward string `json:"forward"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonResponse(w, 400, map[string]any{
				"success": false,
				"errors":  map[string]string{"alias": "invalid request body"},
			})
			return
		}
		state.mu.Lock()
		defer state.mu.Unlock()
		domainAliases, domainExists := state.aliases[domain]
		if !domainExists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "not found"},
			})
			return
		}
		if _, exists := domainAliases[body.Alias]; exists {
			jsonResponse(w, 400, map[string]any{
				"success": false,
				"errors":  map[string]string{"alias": "This alias already exists. You should update the existing one instead."},
			})
			return
		}
		a := &mockAlias{
			ID:      state.nextAliasID,
			Alias:   body.Alias,
			Forward: body.Forward,
		}
		state.nextAliasID++
		domainAliases[body.Alias] = a
		jsonResponse(w, 200, map[string]any{
			"success": true,
			"alias":   a,
		})
	})

	// GET /domains/{domain}/aliases/{alias}
	mux.HandleFunc("GET /domains/{domain}/aliases/{alias}", func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")
		aliasName := r.PathValue("alias")
		state.mu.Lock()
		defer state.mu.Unlock()
		domainAliases, domainExists := state.aliases[domain]
		if !domainExists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "not found"},
			})
			return
		}
		a, ok := domainAliases[aliasName]
		if !ok {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"alias": "not found"},
			})
			return
		}
		jsonResponse(w, 200, map[string]any{
			"success": true,
			"alias":   a,
		})
	})

	// GET /domains/{domain}/aliases
	mux.HandleFunc("GET /domains/{domain}/aliases", func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")
		state.mu.Lock()
		defer state.mu.Unlock()
		domainAliases, domainExists := state.aliases[domain]
		if !domainExists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "not found"},
			})
			return
		}
		list := make([]*mockAlias, 0, len(domainAliases))
		for _, a := range domainAliases {
			list = append(list, a)
		}
		jsonResponse(w, 200, map[string]any{
			"success": true,
			"aliases": list,
		})
	})

	// PUT /domains/{domain}/aliases/{alias}
	mux.HandleFunc("PUT /domains/{domain}/aliases/{alias}", func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")
		aliasName := r.PathValue("alias")
		var body struct {
			Forward string `json:"forward"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		state.mu.Lock()
		defer state.mu.Unlock()
		domainAliases, domainExists := state.aliases[domain]
		if !domainExists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "not found"},
			})
			return
		}
		a, ok := domainAliases[aliasName]
		if !ok {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"alias": "not found"},
			})
			return
		}
		a.Forward = body.Forward
		jsonResponse(w, 200, map[string]any{
			"success": true,
			"alias":   a,
		})
	})

	// DELETE /domains/{domain}/aliases/{alias}
	mux.HandleFunc("DELETE /domains/{domain}/aliases/{alias}", func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")
		aliasName := r.PathValue("alias")
		state.mu.Lock()
		defer state.mu.Unlock()
		domainAliases, domainExists := state.aliases[domain]
		if !domainExists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "not found"},
			})
			return
		}
		if _, ok := domainAliases[aliasName]; !ok {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"alias": "not found"},
			})
			return
		}
		delete(domainAliases, aliasName)
		jsonResponse(w, 200, map[string]any{"success": true})
	})

	// POST /domains/{domain}/credentials
	mux.HandleFunc("POST /domains/{domain}/credentials", func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonResponse(w, 400, map[string]any{
				"success": false,
				"errors":  map[string]string{"credential": "invalid request body"},
			})
			return
		}
		state.mu.Lock()
		defer state.mu.Unlock()
		domainCreds, domainExists := state.credentials[domain]
		if !domainExists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "not found"},
			})
			return
		}
		if _, exists := domainCreds[body.Username]; exists {
			jsonResponse(w, 400, map[string]any{
				"success": false,
				"errors":  map[string]string{"username": "This username already exists for this domain."},
			})
			return
		}
		cred := &mockCredential{
			Username: body.Username,
			Created:  time.Now().Unix(),
		}
		domainCreds[body.Username] = cred
		jsonResponse(w, 200, map[string]any{
			"success":    true,
			"credential": cred,
		})
	})

	// GET /domains/{domain}/credentials
	mux.HandleFunc("GET /domains/{domain}/credentials", func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")
		state.mu.Lock()
		defer state.mu.Unlock()
		domainCreds, domainExists := state.credentials[domain]
		if !domainExists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "not found"},
			})
			return
		}
		list := make([]*mockCredential, 0, len(domainCreds))
		for _, c := range domainCreds {
			list = append(list, c)
		}
		jsonResponse(w, 200, map[string]any{
			"success":     true,
			"credentials": list,
		})
	})

	// PUT /domains/{domain}/credentials/{username}
	mux.HandleFunc("PUT /domains/{domain}/credentials/{username}", func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")
		username := r.PathValue("username")
		var body struct {
			Password string `json:"password"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		state.mu.Lock()
		defer state.mu.Unlock()
		domainCreds, domainExists := state.credentials[domain]
		if !domainExists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "not found"},
			})
			return
		}
		cred, ok := domainCreds[username]
		if !ok {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"username": "not found"},
			})
			return
		}
		// Password is not stored, but we confirm the update succeeded.
		jsonResponse(w, 200, map[string]any{
			"success":    true,
			"credential": cred,
		})
	})

	// DELETE /domains/{domain}/credentials/{username}
	mux.HandleFunc("DELETE /domains/{domain}/credentials/{username}", func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")
		username := r.PathValue("username")
		state.mu.Lock()
		defer state.mu.Unlock()
		domainCreds, domainExists := state.credentials[domain]
		if !domainExists {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"domain": "not found"},
			})
			return
		}
		if _, ok := domainCreds[username]; !ok {
			jsonResponse(w, 404, map[string]any{
				"success": false,
				"errors":  map[string]string{"username": "not found"},
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

// newMockClient returns an ImprovMXClient pointed at the given mock server.
func newMockClient(t *testing.T, server *httptest.Server) *ImprovMXClient {
	t.Helper()
	client := NewImprovMXClient("test-token")
	client.baseURL = server.URL
	return client
}
