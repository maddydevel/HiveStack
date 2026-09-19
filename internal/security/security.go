// Package security provides security hardening for HiveStack on SLES 15 SP7.
//
// This includes:
//   - AppArmor profile generation and management
//   - Audit logging for compliance and forensics
//   - LUKS disk encryption configuration
//   - REST API security middleware (rate limiting, CORS, headers)
package security

// ProfileName represents an AppArmor profile name.
type ProfileName string

const (
	// ManagerProfile is the AppArmor profile name for the Manager service.
	ManagerProfile ProfileName = "hivestack-manager"
	// NodeProfile is the AppArmor profile name for the Node Agent service.
	NodeProfile ProfileName = "hivestack-node"
)

// ServiceType identifies which HiveStack service a security config applies to.
type ServiceType string

const (
	// ServiceManager is the HiveStack Manager.
	ServiceManager ServiceType = "manager"
	// ServiceNode is the HiveStack Node Agent.
	ServiceNode ServiceType = "node"
)
