// Package ha provides Non-HANA High Availability for HiveStack.
//
// Fencing ensures a failed node is truly powered off or isolated before
// VM restart begins, preventing split-brain scenarios.
package ha

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/maddydevel/HiveStack/internal/metrics"
)

// FenceMethod describes the mechanism used to fence a node.
type FenceMethod string

const (
	// FenceMethodIPMI uses IPMI v2.0+ chassis power commands.
	FenceMethodIPMI FenceMethod = "ipmi"
	// FenceMethodRedfish uses the Redfish RESTful API for power control.
	FenceMethodRedfish FenceMethod = "redfish"
	// FenceMethodSSH executes a power-off command via SSH on the BMC/host.
	FenceMethodSSH FenceMethod = "ssh"
)

// String implements fmt.Stringer.
func (f FenceMethod) String() string {
	return string(f)
}

// Fencer defines the interface for node fencing operations.
type Fencer interface {
	// Fence powers off or resets the specified node.
	Fence(ctx context.Context, nodeID string) error
	// GetPowerState returns the current power state ("on", "off", "unknown").
	GetPowerState(ctx context.Context, nodeID string) (string, error)
	// GetMethod returns the fencing method used.
	GetMethod() FenceMethod
}

// FenceConfig holds configuration for a fencing backend.
type FenceConfig struct {
	Method        FenceMethod   `json:"method"`
	Host          string        `json:"host"`           // IPMI/BMC IP, Redfish endpoint, SSH host
	Port          int           `json:"port"`           // Service port
	Username      string        `json:"username"`       // Auth username
	Password      string        `json:"password"`       // Auth password
	SSHKeyPath    string        `json:"ssh_key_path"`   // Path to SSH private key for SSH fencing
	RedfishPath   string        `json:"redfish_path"`   // Redfish Systems endpoint path
	Timeout       time.Duration `json:"timeout"`        // Per-attempt timeout
	MaxRetries    int           `json:"max_retries"`    // Retry attempts
	RetryInterval time.Duration `json:"retry_interval"` // Delay between retries
}

// DefaultFenceConfig returns sensible defaults.
func DefaultFenceConfig() FenceConfig {
	return FenceConfig{
		Method:        FenceMethodIPMI,
		Port:          623,
		Timeout:       30 * time.Second,
		MaxRetries:    3,
		RetryInterval: 10 * time.Second,
		RedfishPath:   "/redfish/v1/Systems/1/Actions/ComputerSystem.Reset",
	}
}

// Validate checks the configuration for required fields.
func (c *FenceConfig) Validate() error {
	switch c.Method {
	case FenceMethodIPMI:
		if c.Host == "" {
			return fmt.Errorf("fencer: IPMI host is required")
		}
		if c.Username == "" || c.Password == "" {
			return fmt.Errorf("fencer: IPMI username and password are required")
		}
	case FenceMethodRedfish:
		if c.Host == "" {
			return fmt.Errorf("fencer: Redfish endpoint is required")
		}
	case FenceMethodSSH:
		if c.Host == "" {
			return fmt.Errorf("fencer: SSH host is required")
		}
	default:
		return fmt.Errorf("fencer: unknown fence method: %s", c.Method)
	}
	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}
	return nil
}

// FenceResult captures the outcome of a fencing operation.
type FenceResult struct {
	NodeID    string        `json:"node_id"`
	Success   bool          `json:"success"`
	Method    FenceMethod   `json:"method"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
}

// MultiFencer tries multiple fencing methods in order of preference.
type MultiFencer struct {
	mu       sync.Mutex
	fencers  []Fencer
	results  []FenceResult
	registry map[string]FenceConfig
}

// NewMultiFencer creates a multi-method fencer from a list of configs.
func NewMultiFencer(configs []FenceConfig) (*MultiFencer, error) {
	fencers := make([]Fencer, 0, len(configs))
	for _, cfg := range configs {
		f, err := NewFencer(cfg)
		if err != nil {
			return nil, fmt.Errorf("create fencer (%s): %w", cfg.Method, err)
		}
		fencers = append(fencers, f)
	}
	return &MultiFencer{
		fencers:  fencers,
		registry: make(map[string]FenceConfig),
	}, nil
}

// NewFencer creates a fencer for the given configuration.
func NewFencer(cfg FenceConfig) (Fencer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	switch cfg.Method {
	case FenceMethodIPMI:
		return &IPMIFencer{config: cfg}, nil
	case FenceMethodRedfish:
		return &RedfishFencer{config: cfg}, nil
	case FenceMethodSSH:
		return &SSHFencer{config: cfg}, nil
	default:
		return nil, fmt.Errorf("unsupported fence method: %s", cfg.Method)
	}
}

// Fence attempts to fence using all configured methods in order.
func (m *MultiFencer) Fence(ctx context.Context, nodeID string) error {
	for _, f := range m.fencers {
		log.Printf("[HA/Fence] Attempting %s fence for node %s", f.GetMethod(), nodeID)

		var err error
		for attempt := 1; attempt <= 3; attempt++ {
			err = f.Fence(ctx, nodeID)
			if err == nil {
				m.recordResult(FenceResult{
					NodeID:    nodeID,
					Success:   true,
					Method:    f.GetMethod(),
					Timestamp: time.Now(),
				})
				log.Printf("[HA/Fence] Node %s successfully fenced via %s (attempt %d)",
					nodeID, f.GetMethod(), attempt)
				return nil
			}
			log.Printf("[HA/Fence] %s fence attempt %d failed for node %s: %v",
				f.GetMethod(), attempt, nodeID, err)
			if attempt < 3 {
				select {
				case <-ctx.Done():
					return fmt.Errorf("fence cancelled: %w", ctx.Err())
				case <-time.After(10 * time.Second):
					// retry
				}
			}
		}

		m.recordResult(FenceResult{
			NodeID:    nodeID,
			Success:   false,
			Method:    f.GetMethod(),
			Error:     err.Error(),
			Timestamp: time.Now(),
		})
	}

	return fmt.Errorf("all fencing methods failed for node %s", nodeID)
}

// GetPowerState returns the power state from the first available method.
func (m *MultiFencer) GetPowerState(ctx context.Context, nodeID string) (string, error) {
	for _, f := range m.fencers {
		state, err := f.GetPowerState(ctx, nodeID)
		if err == nil {
			return state, nil
		}
	}
	return "unknown", fmt.Errorf("could not determine power state for node %s", nodeID)
}

// GetMethod returns the methods as a comma-separated string.
func (m *MultiFencer) GetMethod() FenceMethod {
	methods := make([]string, 0, len(m.fencers))
	for _, f := range m.fencers {
		methods = append(methods, string(f.GetMethod()))
	}
	return FenceMethod(strings.Join(methods, ","))
}

func (m *MultiFencer) recordResult(r FenceResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results = append(m.results, r)
}

// GetResults returns all fencing results.
func (m *MultiFencer) GetResults() []FenceResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]FenceResult{}, m.results...)
}

// ---------- IPMI Fencer ----------

// IPMIFencer implements Fencer using ipmitool for chassis power control.
type IPMIFencer struct {
	config FenceConfig
}

func (f *IPMIFencer) GetMethod() FenceMethod {
	return FenceMethodIPMI
}

func (f *IPMIFencer) Fence(ctx context.Context, nodeID string) error {
	// In production, use the nodeID to look up BMC address from inventory.
	// For now, use the configured host as the BMC address.
	addr := f.config.Host
	cmd := exec.CommandContext(ctx, "ipmitool",
		"-I", "lanplus",
		"-H", addr,
		"-p", fmt.Sprintf("%d", f.config.Port),
		"-U", f.config.Username,
		"-P", f.config.Password,
		"chassis", "power", "off",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ipmitool power off failed: %w (output: %s)", err, string(output))
	}

	log.Printf("[HA/Fence/IPMI] Node %s (%s) powered off: %s", nodeID, addr, strings.TrimSpace(string(output)))
	metrics.RecordFencing(string(FenceMethodIPMI))
	return nil
}

func (f *IPMIFencer) GetPowerState(ctx context.Context, nodeID string) (string, error) {
	cmd := exec.CommandContext(ctx, "ipmitool",
		"-I", "lanplus",
		"-H", f.config.Host,
		"-p", fmt.Sprintf("%d", f.config.Port),
		"-U", f.config.Username,
		"-P", f.config.Password,
		"chassis", "power", "status",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "unknown", fmt.Errorf("ipmitool power status: %w", err)
	}

	out := strings.ToLower(strings.TrimSpace(string(output)))
	switch {
	case strings.Contains(out, "is on"):
		return "on", nil
	case strings.Contains(out, "is off"):
		return "off", nil
	default:
		return "unknown", fmt.Errorf("unexpected power status: %s", out)
	}
}

// ---------- Redfish Fencer ----------

// RedfishFencer implements Fencer using the Redfish REST API.
type RedfishFencer struct {
	config FenceConfig
}

func (f *RedfishFencer) GetMethod() FenceMethod {
	return FenceMethodRedfish
}

func (f *RedfishFencer) Fence(ctx context.Context, nodeID string) error {
	// Use curl to POST a Reset action to the Redfish endpoint.
	// In production, use a proper HTTP client with TLS client cert.
	url := fmt.Sprintf("https://%s:%d%s", f.config.Host, f.config.Port, f.config.RedfishPath)
	resetType := `{"ResetType": "ForceOff"}`

	cmd := exec.CommandContext(ctx, "curl", "-s", "-k", "-X", "POST",
		"-H", "Content-Type: application/json",
		"-d", resetType,
		"-u", fmt.Sprintf("%s:%s", f.config.Username, f.config.Password),
		url,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("redfish power off failed: %w (output: %s)", err, string(output))
	}

	log.Printf("[HA/Fence/Redfish] Node %s (%s) powered off: %s", nodeID, url, string(output))
	metrics.RecordFencing(string(FenceMethodRedfish))
	return nil
}

func (f *RedfishFencer) GetPowerState(ctx context.Context, nodeID string) (string, error) {
	// Query the Systems endpoint to get PowerState
	systemsURL := fmt.Sprintf("https://%s:%d/redfish/v1/Systems/1", f.config.Host, f.config.Port)

	cmd := exec.CommandContext(ctx, "curl", "-s", "-k",
		"-u", fmt.Sprintf("%s:%s", f.config.Username, f.config.Password),
		systemsURL,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "unknown", fmt.Errorf("redfish query: %w", err)
	}

	out := strings.ToLower(string(output))
	if idx := strings.Index(out, `"powerstate"`); idx >= 0 {
		// Extract value
		rest := out[idx:]
		if colonIdx := strings.Index(rest, ":"); colonIdx >= 0 {
			val := rest[colonIdx+1:]
			val = strings.Trim(val, " \",}\n")
			if strings.HasPrefix(val, "on") {
				return "on", nil
			}
			return "off", nil
		}
	}

	return "unknown", fmt.Errorf("could not parse power state from Redfish response")
}

// ---------- SSH Fencer ----------

// SSHFencer implements Fencer by SSH-ing to the host and issuing poweroff.
type SSHFencer struct {
	config FenceConfig
}

func (f *SSHFencer) GetMethod() FenceMethod {
	return FenceMethodSSH
}

func (f *SSHFencer) Fence(ctx context.Context, nodeID string) error {
	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		"-o", "BatchMode=yes",
	}

	if f.config.Port != 0 && f.config.Port != 22 {
		args = append(args, "-p", fmt.Sprintf("%d", f.config.Port))
	}

	if f.config.SSHKeyPath != "" {
		args = append(args, "-i", f.config.SSHKeyPath)
	}

	target := f.config.Host
	if f.config.Username != "" {
		target = f.config.Username + "@" + f.config.Host
	}

	args = append(args, target, "sudo", "poweroff", "-f")

	cmd := exec.CommandContext(ctx, "ssh", args...)
	output, err := cmd.CombinedOutput()
	// SSH connection may be lost when the host powers off — that's expected.
	if err != nil {
		if strings.Contains(string(output), "closed by remote host") ||
			strings.Contains(err.Error(), "signal: killed") ||
			strings.Contains(err.Error(), "exit status 255") {
			log.Printf("[HA/Fence/SSH] Node %s powered off (SSH session terminated)", nodeID)
			metrics.RecordFencing(string(FenceMethodSSH))
			return nil
		}
		return fmt.Errorf("ssh poweroff failed: %w (output: %s)", err, string(output))
	}

	log.Printf("[HA/Fence/SSH] Node %s powered off", nodeID)
	metrics.RecordFencing(string(FenceMethodSSH))
	return nil
}

func (f *SSHFencer) GetPowerState(ctx context.Context, nodeID string) (string, error) {
	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=5",
		"-o", "BatchMode=yes",
	}

	if f.config.Port != 0 && f.config.Port != 22 {
		args = append(args, "-p", fmt.Sprintf("%d", f.config.Port))
	}

	if f.config.SSHKeyPath != "" {
		args = append(args, "-i", f.config.SSHKeyPath)
	}

	target := f.config.Host
	if f.config.Username != "" {
		target = f.config.Username + "@" + f.config.Host
	}

	args = append(args, target, "echo alive")

	cmd := exec.CommandContext(ctx, "ssh", args...)
	err := cmd.Run()
	if err != nil {
		// SSH failure likely means host is off
		return "off", nil
	}

	return "on", nil
}
