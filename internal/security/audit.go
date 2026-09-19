// Package security provides audit logging for HiveStack.
//
// Audit logging captures security-relevant events for compliance and
// forensic analysis. Events are structured JSON and can be forwarded to:
//   - Local syslog (auditd on SLES)
//   - Remote SIEM via TCP/UDP syslog
//   - Files with rotation
//
// Event categories:
//   - Authentication events (login, logout, token refresh)
//   - Authorization events (RBAC denials, privilege escalation)
//   - Data access events (VM creation, storage access)
//   - System events (service start/stop, config changes)
//   - Security events (cert expiry, encryption operations)
package security

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// EventType represents the category of an audit event.
type EventType string

const (
	// AuthEvent covers authentication-related events.
	AuthEvent EventType = "auth"
	// AuthzEvent covers authorization events (RBAC).
	AuthzEvent EventType = "authorization"
	// DataEvent covers data access events.
	DataEvent EventType = "data_access"
	// SystemEvent covers system lifecycle events.
	SystemEvent EventType = "system"
	// SecurityEvent covers security-specific events.
	SecurityEvent EventType = "security"
)

// EventSeverity represents the severity of an audit event.
type EventSeverity string

const (
	// InfoSeverity for routine events.
	InfoSeverity EventSeverity = "info"
	// WarningSeverity for potentially concerning events.
	WarningSeverity EventSeverity = "warning"
	// ErrorSeverity for failures and denials.
	ErrorSeverity EventSeverity = "error"
	// CriticalSeverity for security-critical events.
	CriticalSeverity EventSeverity = "critical"
)

// AuditEvent represents a structured audit log entry.
type AuditEvent struct {
	// ID is the unique event ID.
	ID string `json:"id"`
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`
	// Type is the event category.
	Type EventType `json:"type"`
	// Severity is the event severity.
	Severity EventSeverity `json:"severity"`
	// Actor is who triggered the event (user or service).
	Actor string `json:"actor"`
	// Action is what was attempted.
	Action string `json:"action"`
	// Resource is what was affected.
	Resource string `json:"resource"`
	// Result is whether the action succeeded.
	Result string `json:"result"` // "success" or "failure"
	// Reason provides context for failures.
	Reason string `json:"reason,omitempty"`
	// ClientIP is the source IP if applicable.
	ClientIP string `json:"client_ip,omitempty"`
	// Metadata is additional key-value context.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Logger provides structured audit logging.
type Logger struct {
	logDir    string
	logFile   string
	forwarder LogForwarder
}

// LogForwarder handles forwarding audit events to external systems.
type LogForwarder interface {
	Forward(event *AuditEvent) error
	Close() error
}

// NewLogger creates a new audit logger.
func NewLogger(logDir string) (*Logger, error) {
	if logDir == "" {
		logDir = "/var/log/hivestack/audit"
	}

	if err := os.MkdirAll(logDir, 0750); err != nil {
		return nil, fmt.Errorf("create audit log directory: %w", err)
	}

	return &Logger{
		logDir:  logDir,
		logFile: filepath.Join(logDir, "audit.log"),
	}, nil
}

// Log records an audit event.
func (l *Logger) Log(ctx context.Context, event *AuditEvent) error {
	if event.ID == "" {
		event.ID = generateID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Marshal to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}

	// Write to log file
	if err := l.write(data); err != nil {
		return err
	}

	// Forward to external SIEM if configured
	if l.forwarder != nil {
		if err := l.forwarder.Forward(event); err != nil {
			// Log forwarding errors should not block audit logging
			fmt.Fprintf(os.Stderr, "audit forward error: %v\n", err)
		}
	}

	return nil
}

// write appends a JSON line to the audit log file.
func (l *Logger) write(data []byte) error {
	f, err := os.OpenFile(l.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer f.Close()

	// Append newline after JSON
	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return nil
}

// Close releases resources.
func (l *Logger) Close() error {
	if l.forwarder != nil {
		return l.forwarder.Close()
	}
	return nil
}

// SyslogForwarder forwards audit events to a syslog server.
type SyslogForwarder struct {
	Address string
	Network string // "tcp" or "udp"
}

// Forward sends an event to the syslog server.
func (f *SyslogForwarder) Forward(event *AuditEvent) error {
	// In production, this would connect to the syslog server and forward.
	// For now, this is a stub.
	return nil
}

// Close closes the syslog connection.
func (f *SyslogForwarder) Close() error {
	return nil
}

// generateID generates a unique ID for an audit event.
func generateID() string {
	return fmt.Sprintf("audit-%d-%d", time.Now().UnixNano(), os.Getpid())
}
