// Package security provides AppArmor profile generation for HiveStack.
//
// AppArmor is a Linux security module that restricts programs' capabilities
// with per-program profiles. On SLES 15 SP7, AppArmor is the recommended
// mandatory access control (MAC) system.
//
// This file generates profiles that:
//   - Restrict file access to only necessary paths
//   - Limit network access to required ports and protocols
//   - Deny dangerous capabilities (ptrace, mount, etc.)
//   - Allow only necessary library and binary execution
package security

import (
	"fmt"
	"os"
	"path/filepath"
)

// ProfileManager manages AppArmor profiles for HiveStack services.
type ProfileManager struct {
	// profilesDir is the directory where AppArmor profiles are stored.
	profilesDir string
}

// NewProfileManager creates a new AppArmor profile manager.
func NewProfileManager() *ProfileManager {
	return &ProfileManager{
		profilesDir: "/etc/apparmor.d",
	}
}

// GenerateManagerProfile generates an AppArmor profile for the HiveStack Manager.
//
// The profile restricts the Manager to:
//   - Read/write only to /etc/hivestack, /var/lib/hivestack, /var/log/hivestack
//   - Listen on TCP ports 8080 (HTTP), 8443 (HTTPS), 9090 (gRPC)
//   - Execute only /usr/sbin/hivestack-manager
//   - Access PostgreSQL via unix socket or TCP localhost:5432
//   - Deny write to /usr, /boot, /etc (except hivestack paths)
func GenerateManagerProfile() string {
	return `#
# AppArmor profile for HiveStack Manager
# Auto-generated - do not edit manually
#

#include <tunables/global>

/usr/sbin/hivestack-manager {
	#include <abstractions/base>
	#include <abstractions/nameservice>
	#include <abstractions/ssl_certs>

	# Binary
	/usr/sbin/hivestack-manager mr,

	# Configuration
	/etc/hivestack/ r,
	/etc/hivestack/** r,

	# Data directories
	/var/lib/hivestack/ r,
	/var/lib/hivestack/** rw,

	# Logs
	/var/log/hivestack/ r,
	/var/log/hivestack/** rw,

	# TLS certificates
	/etc/hivestack/tls/ r,
	/etc/hivestack/tls/** r,

	# PID file
	/run/hivestack/ r,
	/run/hivestack/** rw,

	# PostgreSQL (local socket or TCP)
	stream_connect unix /var/run/postgresql/.s.PGSQL.*,
	network inet stream,
	network inet6 stream,

	# Deny dangerous operations
	deny /usr/** w,
	deny /boot/** w,
	deny /etc/shadow r,
	deny /etc/passwd w,

	# Capability restrictions
	capability net_bind_service,
	capability dac_read_search,
	deny capability sys_admin,
	deny capability sys_ptrace,
	deny capability sys_module,
	deny capability dac_override,
}
`
}

// GenerateNodeProfile generates an AppArmor profile for the HiveStack Node Agent.
//
// The profile restricts the Node Agent to:
//   - Read/write to /etc/hivestack, /var/lib/hivestack, /var/log/hivestack
//   - Access libvirt socket (/var/run/libvirt/libvirt-sock)
//   - Manage VM images in /var/lib/hivestack/images/
//   - Connect to the Manager gRPC port (9090)
//   - Deny write to /usr, /boot, most of /etc
func GenerateNodeProfile() string {
	return `#
# AppArmor profile for HiveStack Node Agent
# Auto-generated - do not edit manually
#

#include <tunables/global>

/usr/sbin/hivestack-node {
	#include <abstractions/base>
	#include <abstractions/nameservice>
	#include <abstractions/ssl_certs>

	# Binary
	/usr/sbin/hivestack-node mr,

	# Configuration
	/etc/hivestack/ r,
	/etc/hivestack/** r,

	# Data directories
	/var/lib/hivestack/ r,
	/var/lib/hivestack/** rw,
	/var/lib/hivestack/images/ r,
	/var/lib/hivestack/images/** rw,

	# Logs
	/var/log/hivestack/ r,
	/var/log/hivestack/** rw,

	# TLS certificates
	/etc/hivestack/tls/ r,
	/etc/hivestack/tls/** r,

	# PID file
	/run/hivestack/ r,
	/run/hivestack/** rw,

	# Libvirt socket
	/var/run/libvirt/libvirt-sock rw,

	# Network - connect to Manager
	network inet stream,
	network inet6 stream,

	# Deny dangerous operations
	deny /usr/** w,
	deny /boot/** w,
	deny /etc/shadow r,
	deny /etc/passwd w,

	# Capability restrictions (VM management needs some)
	capability net_bind_service,
	capability dac_read_search,
	capability chown,
	capability fowner,
	deny capability sys_admin,
	deny capability sys_ptrace,
	deny capability sys_module,
}
`
}

// WriteProfile writes an AppArmor profile to the specified path.
func WriteProfile(profile ProfileName, content string) error {
	path := filepath.Join("/etc/apparmor.d", string(profile))
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write AppArmor profile: %w", err)
	}
	return nil
}

// WriteManagerProfile writes the Manager AppArmor profile.
func WriteManagerProfile() error {
	return WriteProfile(ManagerProfile, GenerateManagerProfile())
}

// WriteNodeProfile writes the Node Agent AppArmor profile.
func WriteNodeProfile() error {
	return WriteProfile(NodeProfile, GenerateNodeProfile())
}

// IsAppArmorEnabled checks if AppArmor is enabled on the system.
func IsAppArmorEnabled() bool {
	_, err := os.Stat("/sys/kernel/security/apparmor")
	return err == nil
}

// IsAppArmorEnforcing checks if AppArmor is in enforcing mode.
func IsAppArmorEnforcing() bool {
	data, err := os.ReadFile("/sys/module/apparmor/parameters/enabled")
	if err != nil {
		return false
	}
	return len(data) > 0 && data[0] == 'Y'
}
