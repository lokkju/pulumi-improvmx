package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

// IsAlreadyExists checks if the API error indicates the resource already exists.
func (e *APIError) IsAlreadyExists() bool {
	keywords := []string{"already", "registered", "exists", "update the existing"}
	for _, v := range e.Errors {
		lower := strings.ToLower(v)
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				return true
			}
		}
	}
	lower := strings.ToLower(e.Message)
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// IsNotFound checks if the API error indicates the resource was not found.
func (e *APIError) IsNotFound() bool {
	if e.StatusCode == 404 {
		return true
	}
	keywords := []string{"not found", "not_found", "does not exist"}
	lower := strings.ToLower(e.Message)
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	for _, v := range e.Errors {
		lower := strings.ToLower(v)
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				return true
			}
		}
	}
	return false
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

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, fmt.Errorf("improvmx authentication failed (HTTP %d): check that IMPROVMX_API_TOKEN or improvmx:apiToken is set correctly", resp.StatusCode)
	}

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
