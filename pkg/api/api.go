// Package api provides the HiveStack REST API client.
//
// This client is used by the CLI and other tools to interact with
// the HiveStack Manager's REST API. It handles authentication,
// request construction, and response parsing.
//
// Usage:
//
//	client := api.NewClient("http://localhost:8080", token)
//	vms, err := client.ListVMs(ctx)
//	host, err := client.GetHost(ctx, hostID)
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a REST API client for the HiveStack Manager.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// Option is a functional option for configuring the API client.
type Option func(*Client)

// WithToken sets the authentication token for the client.
func WithToken(token string) Option {
	return func(c *Client) {
		c.token = token
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient = &http.Client{Timeout: d}
	}
}

// NewClient creates a new API client for the HiveStack Manager.
func NewClient(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// SetToken updates the authentication token.
func (c *Client) SetToken(token string) {
	c.token = token
}

// doRequest executes an HTTP request and returns the parsed response.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}
	return nil
}

// doRequestRaw executes an HTTP request and returns the raw response body.
func (c *Client) doRequestRaw(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// Health checks the API server health.
func (c *Client) Health(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.doRequest(ctx, "GET", "/health", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// AuthLogin authenticates a user and returns a JWT token.
func (c *Client) AuthLogin(ctx context.Context, email, password string) (map[string]interface{}, error) {
	body := map[string]string{
		"email":    email,
		"password": password,
	}
	var result map[string]interface{}
	if err := c.doRequest(ctx, "POST", "/api/v1/auth/login", body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// AuthMe returns the current authenticated user.
func (c *Client) AuthMe(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.doRequest(ctx, "GET", "/api/v1/auth/me", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListUsers returns all users in the tenant.
func (c *Client) ListUsers(ctx context.Context) ([]map[string]interface{}, error) {
	data, err := c.doRequestRaw(ctx, "GET", "/api/v1/users", nil)
	if err != nil {
		return nil, err
	}
	var users []map[string]interface{}
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// CreateUser creates a new user.
func (c *Client) CreateUser(ctx context.Context, name, email, password, role string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"name":     name,
		"email":    email,
		"password": password,
		"role":     role,
	}
	var result map[string]interface{}
	if err := c.doRequest(ctx, "POST", "/api/v1/users", body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetUser returns a user by ID.
func (c *Client) GetUser(ctx context.Context, id string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.doRequest(ctx, "GET", "/api/v1/users/"+id, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateUser updates a user.
func (c *Client) UpdateUser(ctx context.Context, id string, updates map[string]interface{}) error {
	return c.doRequest(ctx, "PUT", "/api/v1/users/"+id, updates, nil)
}

// DeleteUser deletes a user by ID.
func (c *Client) DeleteUser(ctx context.Context, id string) error {
	return c.doRequest(ctx, "DELETE", "/api/v1/users/"+id, nil, nil)
}

// ListHosts returns all hosts.
func (c *Client) ListHosts(ctx context.Context) ([]map[string]interface{}, error) {
	data, err := c.doRequestRaw(ctx, "GET", "/api/v1/hosts", nil)
	if err != nil {
		return nil, err
	}
	var hosts []map[string]interface{}
	if err := json.Unmarshal(data, &hosts); err != nil {
		return nil, err
	}
	return hosts, nil
}

// RegisterHost registers a new host.
func (c *Client) RegisterHost(ctx context.Context, name, hostname, ip string) (map[string]interface{}, error) {
	body := map[string]string{
		"name":       name,
		"hostname":   hostname,
		"ip_address": ip,
	}
	var result map[string]interface{}
	if err := c.doRequest(ctx, "POST", "/api/v1/hosts", body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetHost returns a host by ID.
func (c *Client) GetHost(ctx context.Context, id string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.doRequest(ctx, "GET", "/api/v1/hosts/"+id, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateHost updates a host.
func (c *Client) UpdateHost(ctx context.Context, id string, updates map[string]interface{}) error {
	return c.doRequest(ctx, "PUT", "/api/v1/hosts/"+id, updates, nil)
}

// DeleteHost removes a host by ID.
func (c *Client) DeleteHost(ctx context.Context, id string) error {
	return c.doRequest(ctx, "DELETE", "/api/v1/hosts/"+id, nil, nil)
}

// HostStatus returns the status of a host.
func (c *Client) HostStatus(ctx context.Context, id string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.doRequest(ctx, "GET", "/api/v1/hosts/"+id+"/status", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// EnterHostMaintenance puts a host in maintenance mode.
func (c *Client) EnterHostMaintenance(ctx context.Context, id string) error {
	return c.doRequest(ctx, "POST", "/api/v1/hosts/"+id+"/maintenance", nil, nil)
}

// ExitHostMaintenance exits maintenance mode for a host.
func (c *Client) ExitHostMaintenance(ctx context.Context, id string) error {
	return c.doRequest(ctx, "DELETE", "/api/v1/hosts/"+id+"/maintenance", nil, nil)
}

// ListVMs returns all VMs.
func (c *Client) ListVMs(ctx context.Context) ([]map[string]interface{}, error) {
	data, err := c.doRequestRaw(ctx, "GET", "/api/v1/vms", nil)
	if err != nil {
		return nil, err
	}
	var vms []map[string]interface{}
	if err := json.Unmarshal(data, &vms); err != nil {
		return nil, err
	}
	return vms, nil
}

// CreateVM creates a new VM.
func (c *Client) CreateVM(ctx context.Context, vm map[string]interface{}) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.doRequest(ctx, "POST", "/api/v1/vms", vm, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetVM returns a VM by ID.
func (c *Client) GetVM(ctx context.Context, id string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.doRequest(ctx, "GET", "/api/v1/vms/"+id, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateVM updates a VM.
func (c *Client) UpdateVM(ctx context.Context, id string, updates map[string]interface{}) error {
	return c.doRequest(ctx, "PUT", "/api/v1/vms/"+id, updates, nil)
}

// DeleteVM deletes a VM by ID.
func (c *Client) DeleteVM(ctx context.Context, id string) error {
	return c.doRequest(ctx, "DELETE", "/api/v1/vms/"+id, nil, nil)
}

// StartVM starts a VM by ID.
func (c *Client) StartVM(ctx context.Context, id string) error {
	return c.doRequest(ctx, "POST", "/api/v1/vms/"+id+"/start", nil, nil)
}

// StopVM stops a VM by ID.
func (c *Client) StopVM(ctx context.Context, id string) error {
	return c.doRequest(ctx, "POST", "/api/v1/vms/"+id+"/stop", nil, nil)
}

// RestartVM restarts a VM by ID.
func (c *Client) RestartVM(ctx context.Context, id string) error {
	return c.doRequest(ctx, "POST", "/api/v1/vms/"+id+"/restart", nil, nil)
}

// MigrateVM migrates a VM to another host.
func (c *Client) MigrateVM(ctx context.Context, id, hostID string) error {
	body := map[string]string{"host_id": hostID}
	return c.doRequest(ctx, "POST", "/api/v1/vms/"+id+"/migrate", body, nil)
}

// ListVMSnapshots returns snapshots for a VM.
func (c *Client) ListVMSnapshots(ctx context.Context, vmID string) ([]map[string]interface{}, error) {
	data, err := c.doRequestRaw(ctx, "GET", "/api/v1/vms/"+vmID+"/snapshots", nil)
	if err != nil {
		return nil, err
	}
	var snapshots []map[string]interface{}
	if err := json.Unmarshal(data, &snapshots); err != nil {
		return nil, err
	}
	return snapshots, nil
}

// CreateVMSnapshot creates a snapshot for a VM.
func (c *Client) CreateVMSnapshot(ctx context.Context, vmID, name string) (map[string]interface{}, error) {
	body := map[string]string{"name": name}
	var result map[string]interface{}
	if err := c.doRequest(ctx, "POST", "/api/v1/vms/"+vmID+"/snapshots", body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteVMSnapshot deletes a VM snapshot.
func (c *Client) DeleteVMSnapshot(ctx context.Context, vmID, snapshotID string) error {
	return c.doRequest(ctx, "DELETE", "/api/v1/vms/"+vmID+"/snapshots/"+snapshotID, nil, nil)
}

// VMConsole returns the console URL for a VM.
func (c *Client) VMConsole(ctx context.Context, vmID string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.doRequest(ctx, "GET", "/api/v1/vms/"+vmID+"/console", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// VMStats returns statistics for a VM.
func (c *Client) VMStats(ctx context.Context, vmID string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.doRequest(ctx, "GET", "/api/v1/vms/"+vmID+"/stats", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListStoragePools returns all storage pools.
func (c *Client) ListStoragePools(ctx context.Context) ([]map[string]interface{}, error) {
	data, err := c.doRequestRaw(ctx, "GET", "/api/v1/storage-pools", nil)
	if err != nil {
		return nil, err
	}
	var pools []map[string]interface{}
	if err := json.Unmarshal(data, &pools); err != nil {
		return nil, err
	}
	return pools, nil
}

// CreateStoragePool creates a storage pool.
func (c *Client) CreateStoragePool(ctx context.Context, name, poolType, path string) (map[string]interface{}, error) {
	body := map[string]string{
		"name": name,
		"type": poolType,
		"path": path,
	}
	var result map[string]interface{}
	if err := c.doRequest(ctx, "POST", "/api/v1/storage-pools", body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetStoragePool returns a storage pool by ID.
func (c *Client) GetStoragePool(ctx context.Context, id string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.doRequest(ctx, "GET", "/api/v1/storage-pools/"+id, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteStoragePool deletes a storage pool.
func (c *Client) DeleteStoragePool(ctx context.Context, id string) error {
	return c.doRequest(ctx, "DELETE", "/api/v1/storage-pools/"+id, nil, nil)
}

// ListNetworks returns all networks.
func (c *Client) ListNetworks(ctx context.Context) ([]map[string]interface{}, error) {
	data, err := c.doRequestRaw(ctx, "GET", "/api/v1/networks", nil)
	if err != nil {
		return nil, err
	}
	var networks []map[string]interface{}
	if err := json.Unmarshal(data, &networks); err != nil {
		return nil, err
	}
	return networks, nil
}

// CreateNetwork creates a network.
func (c *Client) CreateNetwork(ctx context.Context, net map[string]interface{}) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.doRequest(ctx, "POST", "/api/v1/networks", net, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetNetwork returns a network by ID.
func (c *Client) GetNetwork(ctx context.Context, id string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.doRequest(ctx, "GET", "/api/v1/networks/"+id, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteNetwork deletes a network by ID.
func (c *Client) DeleteNetwork(ctx context.Context, id string) error {
	return c.doRequest(ctx, "DELETE", "/api/v1/networks/"+id, nil, nil)
}

// ListBackups returns all backups.
func (c *Client) ListBackups(ctx context.Context) ([]map[string]interface{}, error) {
	data, err := c.doRequestRaw(ctx, "GET", "/api/v1/backups", nil)
	if err != nil {
		return nil, err
	}
	var backups []map[string]interface{}
	if err := json.Unmarshal(data, &backups); err != nil {
		return nil, err
	}
	return backups, nil
}

// CreateBackup creates a backup.
func (c *Client) CreateBackup(ctx context.Context, vmID, name string) (map[string]interface{}, error) {
	body := map[string]string{
		"vm_id": vmID,
		"name":  name,
	}
	var result map[string]interface{}
	if err := c.doRequest(ctx, "POST", "/api/v1/backups", body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetBackup returns a backup by ID.
func (c *Client) GetBackup(ctx context.Context, id string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.doRequest(ctx, "GET", "/api/v1/backups/"+id, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// RestoreBackup restores a backup.
func (c *Client) RestoreBackup(ctx context.Context, id string) error {
	return c.doRequest(ctx, "POST", "/api/v1/backups/"+id+"/restore", nil, nil)
}

// CancelBackup cancels a backup.
func (c *Client) CancelBackup(ctx context.Context, id string) error {
	return c.doRequest(ctx, "POST", "/api/v1/backups/"+id+"/cancel", nil, nil)
}

// ListEvents returns recent events.
func (c *Client) ListEvents(ctx context.Context) ([]map[string]interface{}, error) {
	data, err := c.doRequestRaw(ctx, "GET", "/api/v1/events", nil)
	if err != nil {
		return nil, err
	}
	var events []map[string]interface{}
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, err
	}
	return events, nil
}
