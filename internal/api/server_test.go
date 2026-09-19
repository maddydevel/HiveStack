// Package api tests the HiveStack REST API server handlers.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/maddydevel/HiveStack/internal/auth"
	"github.com/maddydevel/HiveStack/internal/db"
)

// mockDB implements dbInterface for testing.
type mockDB struct {
	users        []db.User
	hosts        []db.Host
	vms          []db.VM
	pools        []db.StoragePool
	networks     []db.Network
	backups      []db.Backup
	events       []db.Event
	datacenters  []db.Datacenter
	err          map[string]error // optional errors per method name
}

func (m *mockDB) ListUsers(ctx context.Context, tenantID string) ([]db.User, error) {
	if e, ok := m.err["ListUsers"]; ok { return nil, e }
	return m.users, nil
}
func (m *mockDB) CreateUser(ctx context.Context, u *db.User) (string, error) {
	if e, ok := m.err["CreateUser"]; ok { return "", e }
	m.users = append(m.users, *u)
	return "test-user-id", nil
}
func (m *mockDB) GetUser(ctx context.Context, id string) (*db.User, error) {
	if e, ok := m.err["GetUser"]; ok { return nil, e }
	for i := range m.users {
		if m.users[i].ID == id { return &m.users[i], nil }
	}
	return nil, fmt.Errorf("user not found")
}
func (m *mockDB) UpdateUser(ctx context.Context, id string, updates map[string]interface{}) error {
	if e, ok := m.err["UpdateUser"]; ok { return e }
	return nil
}
func (m *mockDB) DeleteUser(ctx context.Context, id string) error {
	if e, ok := m.err["DeleteUser"]; ok { return e }
	for i, u := range m.users {
		if u.ID == id { m.users = append(m.users[:i], m.users[i+1:]...); return nil }
	}
	return fmt.Errorf("user not found")
}

func (m *mockDB) ListHosts(ctx context.Context, tenantID string) ([]db.Host, error) {
	if e, ok := m.err["ListHosts"]; ok { return nil, e }
	return m.hosts, nil
}
func (m *mockDB) CreateHost(ctx context.Context, h *db.Host) (string, error) {
	if e, ok := m.err["CreateHost"]; ok { return "", e }
	m.hosts = append(m.hosts, *h)
	return "test-host-id", nil
}
func (m *mockDB) GetHost(ctx context.Context, id string) (*db.Host, error) {
	if e, ok := m.err["GetHost"]; ok { return nil, e }
	for i := range m.hosts {
		if m.hosts[i].ID == id { return &m.hosts[i], nil }
	}
	return nil, fmt.Errorf("host not found")
}
func (m *mockDB) UpdateHost(ctx context.Context, id string, updates map[string]interface{}) error {
	if e, ok := m.err["UpdateHost"]; ok { return e }
	return nil
}
func (m *mockDB) DeleteHost(ctx context.Context, id string) error {
	if e, ok := m.err["DeleteHost"]; ok { return e }
	return nil
}

func (m *mockDB) ListVMs(ctx context.Context, tenantID string) ([]db.VM, error) {
	if e, ok := m.err["ListVMs"]; ok { return nil, e }
	return m.vms, nil
}
func (m *mockDB) CreateVM(ctx context.Context, vm *db.VM) (string, error) {
	if e, ok := m.err["CreateVM"]; ok { return "", e }
	m.vms = append(m.vms, *vm)
	return "test-vm-id", nil
}
func (m *mockDB) GetVM(ctx context.Context, id string) (*db.VM, error) {
	if e, ok := m.err["GetVM"]; ok { return nil, e }
	for i := range m.vms {
		if m.vms[i].ID == id { return &m.vms[i], nil }
	}
	return nil, fmt.Errorf("vm not found")
}
func (m *mockDB) UpdateVM(ctx context.Context, id string, updates map[string]interface{}) error {
	if e, ok := m.err["UpdateVM"]; ok { return e }
	return nil
}
func (m *mockDB) DeleteVM(ctx context.Context, id string) error {
	if e, ok := m.err["DeleteVM"]; ok { return e }
	return nil
}

func (m *mockDB) ListStoragePools(ctx context.Context, tenantID string) ([]db.StoragePool, error) {
	if e, ok := m.err["ListStoragePools"]; ok { return nil, e }
	return m.pools, nil
}
func (m *mockDB) CreateStoragePool(ctx context.Context, sp *db.StoragePool) (string, error) {
	if e, ok := m.err["CreateStoragePool"]; ok { return "", e }
	m.pools = append(m.pools, *sp)
	return "test-pool-id", nil
}
func (m *mockDB) GetStoragePool(ctx context.Context, id string) (*db.StoragePool, error) {
	if e, ok := m.err["GetStoragePool"]; ok { return nil, e }
	for i := range m.pools {
		if m.pools[i].ID == id { return &m.pools[i], nil }
	}
	return nil, fmt.Errorf("pool not found")
}
func (m *mockDB) DeleteStoragePool(ctx context.Context, id string) error {
	if e, ok := m.err["DeleteStoragePool"]; ok { return e }
	return nil
}

func (m *mockDB) ListNetworks(ctx context.Context, tenantID string) ([]db.Network, error) {
	if e, ok := m.err["ListNetworks"]; ok { return nil, e }
	return m.networks, nil
}
func (m *mockDB) CreateNetwork(ctx context.Context, n *db.Network) (string, error) {
	if e, ok := m.err["CreateNetwork"]; ok { return "", e }
	m.networks = append(m.networks, *n)
	return "test-net-id", nil
}
func (m *mockDB) GetNetwork(ctx context.Context, id string) (*db.Network, error) {
	if e, ok := m.err["GetNetwork"]; ok { return nil, e }
	for i := range m.networks {
		if m.networks[i].ID == id { return &m.networks[i], nil }
	}
	return nil, fmt.Errorf("network not found")
}
func (m *mockDB) DeleteNetwork(ctx context.Context, id string) error {
	if e, ok := m.err["DeleteNetwork"]; ok { return e }
	return nil
}

func (m *mockDB) ListBackups(ctx context.Context, tenantID string) ([]db.Backup, error) {
	if e, ok := m.err["ListBackups"]; ok { return nil, e }
	return m.backups, nil
}
func (m *mockDB) CreateBackup(ctx context.Context, b *db.Backup) (string, error) {
	if e, ok := m.err["CreateBackup"]; ok { return "", e }
	m.backups = append(m.backups, *b)
	return "test-backup-id", nil
}
func (m *mockDB) GetBackup(ctx context.Context, id string) (*db.Backup, error) {
	if e, ok := m.err["GetBackup"]; ok { return nil, e }
	for i := range m.backups {
		if m.backups[i].ID == id { return &m.backups[i], nil }
	}
	return nil, fmt.Errorf("backup not found")
}
func (m *mockDB) UpdateBackup(ctx context.Context, id string, updates map[string]interface{}) error {
	if e, ok := m.err["UpdateBackup"]; ok { return e }
	return nil
}

func (m *mockDB) ListEvents(ctx context.Context, tenantID string, limit int) ([]db.Event, error) {
	if e, ok := m.err["ListEvents"]; ok { return nil, e }
	return m.events, nil
}

func (m *mockDB) ListDatacenters(ctx context.Context, tenantID string) ([]db.Datacenter, error) {
	if e, ok := m.err["ListDatacenters"]; ok { return nil, e }
	return m.datacenters, nil
}
func (m *mockDB) CreateDatacenter(ctx context.Context, dc *db.Datacenter) (string, error) {
	if e, ok := m.err["CreateDatacenter"]; ok { return "", e }
	m.datacenters = append(m.datacenters, *dc)
	return "test-dc-id", nil
}
func (m *mockDB) GetDatacenter(ctx context.Context, id string) (*db.Datacenter, error) {
	if e, ok := m.err["GetDatacenter"]; ok { return nil, e }
	for i := range m.datacenters {
		if m.datacenters[i].ID == id { return &m.datacenters[i], nil }
	}
	return nil, fmt.Errorf("datacenter not found")
}
func (m *mockDB) DeleteDatacenter(ctx context.Context, id string) error {
	if e, ok := m.err["DeleteDatacenter"]; ok { return e }
	return nil
}

// makeToken generates a valid JWT for testing.
// The caller must set HIVESTACK_JWT_SECRET before calling (each test function
// does this via os.Setenv + defer os.Unsetenv).
func makeToken(t *testing.T, userID, tenantID string, scopes []string) string {
	t.Helper()
	if os.Getenv("HIVESTACK_JWT_SECRET") == "" {
		t.Fatal("HIVESTACK_JWT_SECRET not set — call os.Setenv before makeToken")
	}
	token, err := auth.GenerateToken(userID, tenantID, scopes, 0)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	return token
}

// newTestAPI creates an APIServer with a mock DB for testing.
// It constructs the server directly (bypassing New()) since New() requires *db.DB.
func newTestAPI(t *testing.T, mdb *mockDB) *APIServer {
	t.Helper()
	cfg := &Config{
		Server: ServerConfig{Host: "localhost", Port: 8080},
		Auth:   AuthConfig{JWTSecret: "test-secret"},
	}
	s := &APIServer{
		Config:     cfg,
		db:         mdb,
		rbac:       auth.NewRBACEngine(),
		mux:        new(http.ServeMux),
		vmHandler:  &mockVMHandler{},
	}
	s.registerRoutes()
	return s
}

// mockVMHandler implements VMHandler for testing.
type mockVMHandler struct{}

func (h *mockVMHandler) MigrateVM(ctx context.Context, id, targetHostID string) error {
	return nil
}

func (h *mockVMHandler) GetSnapshots(ctx context.Context, vmID string) ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

func (h *mockVMHandler) CreateSnapshot(ctx context.Context, vmID, name string) (string, error) {
	return "snap-test", nil
}

func (h *mockVMHandler) DeleteSnapshot(ctx context.Context, vmID, snapshotID string) error {
	return nil
}

func (h *mockVMHandler) GetVMStats(ctx context.Context, vmID string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"cpu_usage":   0,
		"memory_usage": 0,
		"disk_io":     0,
		"network_io":  0,
	}, nil
}

// authRequest returns a request with a Bearer token.
func authRequest(method, path, token string, body interface{}) *http.Request {
	var r *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		r = httptest.NewRequest(method, path, bytes.NewReader(b))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	return r
}

// requireAuth returns the expected 401 response body.
var requireAuthBody = map[string]string{"error": "unauthorized", "code": "UNAUTHORIZED"}

func TestHandleHealth(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	mdb := &mockDB{}
		srv := newTestAPI(t, mdb)
		defer func() {}()

	// Health endpoint has no auth
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("health: got %d, want %d", rec.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("status: got %v, want ok", resp["status"])
	}
	if resp["version"] != Version {
		t.Errorf("version: got %v, want %v", resp["version"], Version)
	}
}

func TestHandleLogin(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	mdb := &mockDB{}
		srv := newTestAPI(t, mdb)
		defer func() {}()

	// Missing body
	req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing body: got %d, want %d", rec.Code, http.StatusBadRequest)
	}

	// Valid login
	req = httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader([]byte(`{"email":"test@test.com","password":"pass"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("login: got %d, want %d", rec.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["token"] == nil {
		t.Error("missing token in login response")
	}
}

func TestHandleMe(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	mdb := &mockDB{}
		srv := newTestAPI(t, mdb)
		defer func() {}()

	// Unauthorized
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthorized: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	// Authenticated
	token := makeToken(t, "user-1", "tenant-1", []string{"viewer"})
	req = authRequest("GET", "/api/v1/auth/me", token, nil)
	rec = httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("authenticated: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleUsersCRUD(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	t.Run("ListUsers unauthorized", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		req := httptest.NewRequest("GET", "/api/v1/users", nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rec.Code)
		}
	})

	t.Run("ListUsers authenticated", func(t *testing.T) {
		mdb := &mockDB{users: []db.User{{ID: "u1", TenantID: "t1", Name: "Alice", Email: "a@test.com", Role: "admin"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"viewer"})
		req := authRequest("GET", "/api/v1/users", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})

	t.Run("CreateUser unauthorized", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		req := authRequest("POST", "/api/v1/users", "", nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rec.Code)
		}
	})

	t.Run("CreateUser missing fields", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		req := authRequest("POST", "/api/v1/users", token, map[string]string{"name": ""})
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("got %d, want 400", rec.Code)
		}
	})

	t.Run("CreateUser success", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		body := map[string]string{"name": "Bob", "email": "b@test.com", "password": "secret", "role": "viewer"}
		req := authRequest("POST", "/api/v1/users", token, body)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Errorf("got %d, want 201", rec.Code)
		}
	})

	t.Run("GetUser not found", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"viewer"})
		req := authRequest("GET", "/api/v1/users/nonexistent", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("got %d, want 404", rec.Code)
		}
	})

	t.Run("GetUser success", func(t *testing.T) {
		mdb := &mockDB{users: []db.User{{ID: "u1", TenantID: "t1", Name: "Alice", Email: "a@test.com", Role: "admin"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"viewer"})
		req := authRequest("GET", "/api/v1/users/u1", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
		var resp db.User
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp.PasswordHash != "" {
			t.Error("PasswordHash should be cleared")
		}
	})

	t.Run("UpdateUser not found", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		body := map[string]string{"name": "NewName"}
		req := authRequest("PUT", "/api/v1/users/nonexistent", token, body)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("got %d, want 404", rec.Code)
		}
	})

	t.Run("UpdateUser success", func(t *testing.T) {
		mdb := &mockDB{users: []db.User{{ID: "u1", TenantID: "t1", Name: "Alice", Email: "a@test.com", Role: "admin"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		body := map[string]string{"name": "NewName"}
		req := authRequest("PUT", "/api/v1/users/u1", token, body)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})

	t.Run("DeleteUser not found", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		req := authRequest("DELETE", "/api/v1/users/nonexistent", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("got %d, want 404", rec.Code)
		}
	})

	t.Run("DeleteUser success", func(t *testing.T) {
		mdb := &mockDB{users: []db.User{{ID: "u1", TenantID: "t1", Name: "Alice", Email: "a@test.com", Role: "admin"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		req := authRequest("DELETE", "/api/v1/users/u1", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})
}

func TestHandleHostsCRUD(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	t.Run("ListHosts unauthorized", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		req := httptest.NewRequest("GET", "/api/v1/hosts", nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rec.Code)
		}
	})

	t.Run("ListHosts authenticated", func(t *testing.T) {
		mdb := &mockDB{hosts: []db.Host{{ID: "h1", TenantID: "t1", Name: "host1", Hostname: "h1.local", Status: "online"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"viewer"})
		req := authRequest("GET", "/api/v1/hosts", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})

	t.Run("RegisterHost success", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		body := map[string]string{"name": "new-host", "hostname": "nh.local", "ip_address": "10.0.0.1"}
		req := authRequest("POST", "/api/v1/hosts", token, body)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Errorf("got %d, want 201", rec.Code)
		}
	})

	t.Run("GetHost not found", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"viewer"})
		req := authRequest("GET", "/api/v1/hosts/nonexistent", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("got %d, want 404", rec.Code)
		}
	})

	t.Run("GetHost success", func(t *testing.T) {
		mdb := &mockDB{hosts: []db.Host{{ID: "h1", TenantID: "t1", Name: "host1", Hostname: "h1.local", Status: "online"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"viewer"})
		req := authRequest("GET", "/api/v1/hosts/h1", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})

	t.Run("UpdateHost success", func(t *testing.T) {
		mdb := &mockDB{hosts: []db.Host{{ID: "h1", TenantID: "t1", Name: "host1", Hostname: "h1.local", Status: "online"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		body := map[string]string{"name": "renamed", "status": "maintenance"}
		req := authRequest("PUT", "/api/v1/hosts/h1", token, body)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})

	t.Run("DeleteHost success", func(t *testing.T) {
		mdb := &mockDB{hosts: []db.Host{{ID: "h1", TenantID: "t1", Name: "host1", Hostname: "h1.local", Status: "online"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		req := authRequest("DELETE", "/api/v1/hosts/h1", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})
}

func TestHandleVMsCRUD(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	t.Run("ListVMs unauthorized", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		req := httptest.NewRequest("GET", "/api/v1/vms", nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rec.Code)
		}
	})

	t.Run("CreateVM unauthorized", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		req := authRequest("POST", "/api/v1/vms", "", nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rec.Code)
		}
	})

	t.Run("CreateVM missing name", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		body := map[string]interface{}{"cpus": 4, "memory_bytes": int64(8589934592)}
		req := authRequest("POST", "/api/v1/vms", token, body)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("got %d, want 400", rec.Code)
		}
	})

	t.Run("CreateVM success", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		body := map[string]interface{}{
			"name": "test-vm", "cpus": 4, "memory_bytes": int64(8589934592),
			"cpu_allocation": "dedicated", "role": "generic",
		}
		req := authRequest("POST", "/api/v1/vms", token, body)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Errorf("got %d, want 201", rec.Code)
		}
	})

	t.Run("GetVM not found", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"viewer"})
		req := authRequest("GET", "/api/v1/vms/nonexistent", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("got %d, want 404", rec.Code)
		}
	})

	t.Run("GetVM success", func(t *testing.T) {
		mdb := &mockDB{vms: []db.VM{{ID: "vm1", TenantID: "t1", Name: "test-vm", CPUs: 4, MemoryBytes: 8589934592, Role: db.VMRoleGeneric, Status: "running"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"viewer"})
		req := authRequest("GET", "/api/v1/vms/vm1", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})

	t.Run("UpdateVM not found", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		body := map[string]string{"name": "new-name"}
		req := authRequest("PUT", "/api/v1/vms/nonexistent", token, body)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("got %d, want 404", rec.Code)
		}
	})

	t.Run("UpdateVM success", func(t *testing.T) {
		mdb := &mockDB{vms: []db.VM{{ID: "vm1", TenantID: "t1", Name: "test-vm", CPUs: 4, MemoryBytes: 8589934592, Role: db.VMRoleGeneric, Status: "running"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		body := map[string]string{"name": "renamed-vm", "status": "stopped"}
		req := authRequest("PUT", "/api/v1/vms/vm1", token, body)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})

	t.Run("DeleteVM success", func(t *testing.T) {
		mdb := &mockDB{vms: []db.VM{{ID: "vm1", TenantID: "t1", Name: "test-vm", CPUs: 4, MemoryBytes: 8589934592, Role: db.VMRoleGeneric, Status: "running"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		req := authRequest("DELETE", "/api/v1/vms/vm1", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})

	t.Run("VMStart success", func(t *testing.T) {
		mdb := &mockDB{vms: []db.VM{{ID: "vm1", TenantID: "t1", Name: "test-vm", CPUs: 4, MemoryBytes: 8589934592, Role: db.VMRoleGeneric, Status: "stopped"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		req := authRequest("POST", "/api/v1/vms/vm1/start", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})

	t.Run("VMStop success", func(t *testing.T) {
		mdb := &mockDB{vms: []db.VM{{ID: "vm1", TenantID: "t1", Name: "test-vm", CPUs: 4, MemoryBytes: 8589934592, Role: db.VMRoleGeneric, Status: "running"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		req := authRequest("POST", "/api/v1/vms/vm1/stop", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})

	t.Run("VMMigrate success", func(t *testing.T) {
		mdb := &mockDB{vms: []db.VM{{ID: "vm1", TenantID: "t1", Name: "test-vm", CPUs: 4, MemoryBytes: 8589934592, Role: db.VMRoleGeneric, Status: "running"}}}
		srv := newTestAPI(t, mdb)
		defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		body := strings.NewReader(`{"target_host": "host2"}`)
		req := httptest.NewRequest("POST", "/api/v1/vms/vm1/migrate", body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Errorf("got %d, want %d", rec.Code, http.StatusAccepted)
		}
	})
}

func TestHandleStoragePoolsCRUD(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	t.Run("ListStoragePools unauthorized", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		req := httptest.NewRequest("GET", "/api/v1/storage-pools", nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rec.Code)
		}
	})

	t.Run("CreateStoragePool success", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		body := map[string]string{"name": "pool1", "type": "directory", "path": "/data"}
		req := authRequest("POST", "/api/v1/storage-pools", token, body)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Errorf("got %d, want 201", rec.Code)
		}
	})

	t.Run("GetStoragePool not found", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"viewer"})
		req := authRequest("GET", "/api/v1/storage-pools/nonexistent", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("got %d, want 404", rec.Code)
		}
	})

	t.Run("DeleteStoragePool success", func(t *testing.T) {
		mdb := &mockDB{pools: []db.StoragePool{{ID: "p1", TenantID: "t1", Name: "pool1", Type: "directory", Status: "active"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		req := authRequest("DELETE", "/api/v1/storage-pools/p1", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})
}

func TestHandleNetworksCRUD(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	t.Run("CreateNetwork success", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		body := map[string]string{"name": "net1", "type": "bridge", "bridge_name": "br0", "subnet": "10.0.0.0/24"}
		req := authRequest("POST", "/api/v1/networks", token, body)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Errorf("got %d, want 201", rec.Code)
		}
	})

	t.Run("GetNetwork not found", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"viewer"})
		req := authRequest("GET", "/api/v1/networks/nonexistent", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("got %d, want 404", rec.Code)
		}
	})

	t.Run("DeleteNetwork success", func(t *testing.T) {
		mdb := &mockDB{networks: []db.Network{{ID: "n1", TenantID: "t1", Name: "net1", Type: "bridge", Status: "active"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		req := authRequest("DELETE", "/api/v1/networks/n1", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})
}

func TestHandleBackupsCRUD(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	t.Run("ListBackups unauthorized", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		req := httptest.NewRequest("GET", "/api/v1/backups", nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rec.Code)
		}
	})

	t.Run("CreateBackup missing fields", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		body := map[string]string{"name": "backup1"}
		req := authRequest("POST", "/api/v1/backups", token, body)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("got %d, want 400", rec.Code)
		}
	})

	t.Run("CreateBackup success", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		body := map[string]string{"vm_id": "vm1", "name": "backup1", "type": "full", "storage_path": "/backups"}
		req := authRequest("POST", "/api/v1/backups", token, body)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Errorf("got %d, want 201", rec.Code)
		}
	})

	t.Run("GetBackup not found", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"viewer"})
		req := authRequest("GET", "/api/v1/backups/nonexistent", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("got %d, want 404", rec.Code)
		}
	})

	t.Run("BackupRestore success", func(t *testing.T) {
		mdb := &mockDB{backups: []db.Backup{{ID: "b1", TenantID: "t1", VMID: "vm1", Name: "backup1", Status: "completed"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		req := authRequest("POST", "/api/v1/backups/b1/restore", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})

	t.Run("BackupCancel success", func(t *testing.T) {
		mdb := &mockDB{backups: []db.Backup{{ID: "b1", TenantID: "t1", VMID: "vm1", Name: "backup1", Status: "creating"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		req := authRequest("POST", "/api/v1/backups/b1/cancel", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})
}

func TestHandleEvents(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	t.Run("ListEvents unauthorized", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		req := httptest.NewRequest("GET", "/api/v1/events", nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rec.Code)
		}
	})

	t.Run("ListEvents authenticated", func(t *testing.T) {
		mdb := &mockDB{events: []db.Event{{ID: "e1", TenantID: "t1", Type: "vm_created", Severity: "info", Message: "VM created"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"viewer"})
		req := authRequest("GET", "/api/v1/events", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})
}

func TestHandleCompliance(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	t.Run("ComplianceCheck unauthorized", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		req := httptest.NewRequest("GET", "/api/v1/compliance/vms/vm1", nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rec.Code)
		}
	})

	t.Run("ComplianceCheck vm not found", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"viewer"})
		req := authRequest("GET", "/api/v1/compliance/vms/nonexistent", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("got %d, want 404", rec.Code)
		}
	})

	t.Run("ComplianceCheck generic VM passes", func(t *testing.T) {
		mdb := &mockDB{vms: []db.VM{{ID: "vm1", TenantID: "t1", Name: "test-vm", CPUs: 4, MemoryBytes: 8589934592, Role: db.VMRoleGeneric, Status: "running"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"viewer"})
		req := authRequest("GET", "/api/v1/compliance/vms/vm1", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp["compliant"] != true {
			t.Error("generic VM should be compliant")
		}
	})

	t.Run("ComplianceEvidence unauthorized", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		req := httptest.NewRequest("GET", "/api/v1/compliance/evidence/vm1", nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rec.Code)
		}
	})

	t.Run("ComplianceDrift unauthorized", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		req := httptest.NewRequest("GET", "/api/v1/compliance/drift", nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rec.Code)
		}
	})
}

func TestHandleLogout(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	mdb := &mockDB{}
		srv := newTestAPI(t, mdb)
		defer func() {}()

	token := makeToken(t, "u1", "t1", []string{"viewer"})
	req := authRequest("POST", "/api/v1/auth/logout", token, nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got %d, want 200", rec.Code)
	}
}

func TestHandleDatacentersClusters(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	t.Run("ListDCs unauthorized", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		req := httptest.NewRequest("GET", "/api/v1/datacenters", nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rec.Code)
		}
	})

	t.Run("ListDCs authenticated returns empty", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"viewer"})
		req := authRequest("GET", "/api/v1/datacenters", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})

	t.Run("CreateDC unauthorized", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		req := httptest.NewRequest("POST", "/api/v1/datacenters", nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rec.Code)
		}
	})

	t.Run("CreateDC success", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		body := map[string]string{"name": "dc1", "description": "main datacenter"}
		req := authRequest("POST", "/api/v1/datacenters", token, body)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Errorf("got %d, want 201", rec.Code)
		}
	})

	// Invalid ID: route won't match empty segment, so we test with a non-existent ID
	// which correctly returns 404 (not found) rather than 400.
	// (Go 1.22 ServeMux {id} does not match empty path segments.)
	t.Run("GetDC not found", func(t *testing.T) {
		mdb := &mockDB{}
		srv := newTestAPI(t, mdb)
		defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"viewer"})
		req := authRequest("GET", "/api/v1/datacenters/nonexistent", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("got %d, want 404", rec.Code)
		}
	})

	t.Run("DeleteDC success", func(t *testing.T) {
		mdb := &mockDB{}
		srv := newTestAPI(t, mdb)
		defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		req := authRequest("DELETE", "/api/v1/datacenters/dc1", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})

	t.Run("ListClusters unauthorized", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		req := httptest.NewRequest("GET", "/api/v1/clusters", nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rec.Code)
		}
	})

	t.Run("CreateCluster success", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		body := map[string]string{"name": "cluster1", "description": "main cluster", "datacenter_id": "dc1"}
		req := authRequest("POST", "/api/v1/clusters", token, body)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Errorf("got %d, want 201", rec.Code)
		}
	})
}

func TestHandleHostMaintenance(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	t.Run("EnterMaintenance unauthorized", func(t *testing.T) {
		mdb := &mockDB{}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		req := httptest.NewRequest("POST", "/api/v1/hosts/h1/maintenance", nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", rec.Code)
		}
	})

	t.Run("EnterMaintenance success", func(t *testing.T) {
		mdb := &mockDB{hosts: []db.Host{{ID: "h1", TenantID: "t1", Name: "host1", Hostname: "h1.local", Status: "online"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		req := authRequest("POST", "/api/v1/hosts/h1/maintenance", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})

	t.Run("ExitMaintenance success", func(t *testing.T) {
		mdb := &mockDB{hosts: []db.Host{{ID: "h1", TenantID: "t1", Name: "host1", Hostname: "h1.local", Status: "maintenance"}}}
			srv := newTestAPI(t, mdb)
			defer func() {}()
		token := makeToken(t, "u1", "t1", []string{"admin"})
		req := authRequest("DELETE", "/api/v1/hosts/h1/maintenance", token, nil)
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("got %d, want 200", rec.Code)
		}
	})
}
