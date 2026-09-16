package db

import "time"

// Tenant is a customer/organization boundary. Every tenant-scoped table
// carries a tenant_id FK so a single HiveStack Manager instance can safely
// serve multiple customers.
type Tenant struct {
	ID        string
	Name      string
	Slug      string
	Status    string
	Config    []byte // raw JSON
	CreatedAt time.Time
	UpdatedAt time.Time
}

// User is a tenant-scoped operator/administrator account.
type User struct {
	ID           string
	TenantID     string
	Name         string
	Email        string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// APIToken is a tenant-scoped, scope-limited bearer token that can expire.
type APIToken struct {
	ID          string
	TenantID    string
	UserID      string
	Name        string
	TokenHash   string
	Scopes      []string
	ExpiresAt   *time.Time
	LastUsedAt  *time.Time
	CreatedAt   time.Time
}

// Datacenter is a top-level tenant-scoped inventory grouping.
type Datacenter struct {
	ID          string
	TenantID    string
	Name        string
	Description string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Cluster groups hosts within a datacenter.
type Cluster struct {
	ID           string
	TenantID     string
	DatacenterID string
	Name         string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Host is a physical KVM hypervisor node.
type Host struct {
	ID                string
	TenantID          string
	ClusterID         *string
	Name              string
	Hostname          string
	IPAddress         string
	Status            string
	CPUModel          string
	CPUCount          int
	MemoryTotalBytes  int64
	StorageTotalBytes int64
	OS                string
	Hypervisor        string
	AgentTokenHash    string
	NUMANodeCount     int
	HugepagesTotalKB  int64
	MaintenanceMode   bool
	JoinedAt          *time.Time
	LastHeartbeat     *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// VMRole distinguishes generic workloads from SAP HANA workloads, which are
// subject to guardrail validation (see internal/compliance).
type VMRole string

const (
	VMRoleGeneric VMRole = "generic"
	VMRoleHANA    VMRole = "hana"
)

// VM is a virtual machine. The NUMA/hugepage/pinning/reservation/ballooning/
// swap columns are the SAP HANA guardrail fields: for VMRoleHANA VMs they
// must reflect a NUMA-pinned, hugepage-backed, non-overcommitted
// configuration; for generic VMs they are typically left at permissive
// defaults.
type VM struct {
	ID              string
	TenantID        string
	ClusterID       *string
	HostID          *string
	Identifier      string
	Name            string
	Description     string
	Status          string
	Role            VMRole
	CPUs            int
	CPUAllocation   string
	MemoryBytes     int64

	NUMAPolicy              *string
	HugepagesEnabled        bool
	CPUPinning              []byte // raw JSON, e.g. {"vcpu0":"0",...}
	MemoryReservationBytes  int64
	BallooningAllowed       bool
	SwapAllowed             bool

	OS             string
	TemplateID     *string
	SnapshotCount  int
	CreatedAt      time.Time
	StartedAt      *time.Time
	UpdatedAt      time.Time
}

// StoragePool is a tenant-scoped storage backend.
type StoragePool struct {
	ID                 string
	TenantID           string
	DatacenterID       *string
	Name               string
	Type               string
	Path               string
	Status             string
	TotalCapacityBytes int64
	FreeSpaceBytes     int64
	UsedSpaceBytes     int64
	Features           []string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Network is a tenant-scoped virtual network definition.
type Network struct {
	ID           string
	TenantID     string
	DatacenterID *string
	Name         string
	Description  string
	Type         string
	BridgeName   string
	VLANID       *int
	Subnet       string
	Gateway      string
	DHCP         bool
	DNS          []string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Disk is a virtual disk, optionally attached to a VM.
type Disk struct {
	ID             string
	TenantID       string
	VMID           *string
	StoragePoolID  *string
	Name           string
	SizeBytes      int64
	Format         string
	Path           string
	Bus            string
	Mounted        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Backup is a point-in-time backup job/record for a VM.
type Backup struct {
	ID           string
	TenantID     string
	VMID         string
	Name         string
	Status       string
	Type         string
	StoragePath  string
	SizeBytes    int64
	Progress     int
	Message      string
	ScheduleID   *string
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
}

// Snapshot is a VM disk-state snapshot.
type Snapshot struct {
	ID          string
	TenantID    string
	VMID        string
	Name        string
	Description string
	State       string
	SizeBytes   int64
	CreatedAt   time.Time
}

// ComplianceEvidence is one hash-chained compliance check result for a VM.
// previous_hash links to the prior evidence row for the same VM (or a fixed
// genesis value for the first row); hash commits to previous_hash plus this
// row's own content, making the chain tamper-evident.
type ComplianceEvidence struct {
	ID            string
	TenantID      string
	VMID          string
	CheckType     string
	CheckResult   []byte // raw JSON
	Passed        bool
	Evidence      []byte // raw JSON
	PreviousHash  string
	Hash          string
	CreatedAt     time.Time
}

// Event is a tenant-scoped, append-only audit log entry.
type Event struct {
	ID           string
	TenantID     string
	Type         string
	Severity     string
	Message      string
	ActorType    string
	ActorID      string
	ActorName    string
	ResourceType string
	ResourceID   string
	ResourceName string
	Metadata     []byte // raw JSON
	CreatedAt    time.Time
}
