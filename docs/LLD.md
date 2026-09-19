# HiveStack Low-Level Design (LLD)

## Package Dependencies

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              api                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │ server.go    │  │ migration.go │  │ ovf.go       │  │            │ │
│  │              │  │              │  │              │  │            │ │
│  │ HTTP handlers│  │ Job mgmt     │  │ OVF parsing  │  │            │ │
│  └──────┬───────┘  └──────┬───────┘  └──────────────┘  └────────────┘ │
│         │                  │                                            │
│         │ uses             │ uses                                       │
│         ▼                  ▼                                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │ vcenter      │  │ preflight    │  │ migration    │  │            │ │
│  │              │  │              │  │              │  │            │ │
│  │ Client       │  │ Checker      │  │ Tracker      │  │            │ │
│  │ Discovery    │  │ Validation   │  │ JobExecutor  │  │            │ │
│  └──────────────┘  └──────────────┘  └──────────────┘  └────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                              ha                                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │ controller.go│  │orchestrator.go│ │ fencer.go    │  │ health.go  │ │
│  │              │  │              │  │              │  │            │ │
│  │ Main loop    │  │ Failover     │  │ IPMI/Redfish/│  │ Heartbeat  │ │
│  │ Reconcile    │  │ Pipeline     │  │ SSH fencing  │  │ Processing │ │
│  └──────┬───────┘  └──────┬───────┘  └──────────────┘  └────────────┘ │
│         │                  │                                            │
│         │ depends on       │ depends on                                 │
│         ▼                  ▼                                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │ scheduler.go │  │ policy.go    │  │ storage.go   │  │            │ │
│  │              │  │              │  │              │  │            │ │
│  │ Host select  │  │ Per-VM HA    │  │ NFS/Ceph/    │  │            │ │
│  │ Scoring      │  │ Policy mgmt  │  │ iSCSI        │  │            │ │
│  └──────────────┘  └──────────────┘  └──────────────┘  └────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

## Class Diagrams

### HA Controller Subsystem

```
┌─────────────────────────┐         ┌─────────────────────────┐
│     Controller          │         │   ControllerConfig      │
├─────────────────────────┤         ├─────────────────────────┤
│ - mu: sync.Mutex        │         │ - HeartbeatProcessor    │
│ - processor: HBProc     │         │ - Orchestrator          │
│ - orchestrator: Orch    │         │ - Fencer                │
│ - fencer: Fencer        │         │ - Scheduler             │
│ - scheduler: Scheduler  │         │ - Threshold             │
│ - running: bool         │         │ - FailoverTimeout       │
│ - cancel: CancelFunc    │         │ - MetricsCallback       │
│ - threshold: Thresholds │         │ - EventCallback         │
│ - failoverTimeout: time │         └─────────────────────────┘
│ - metricsCallback: func │
│ - eventCallback: func   │
├─────────────────────────┤
│ + NewController(cfg)    │
│ + Start(ctx) error      │
│ + Stop() error          │
│ - mainLoop(ctx)         │
│ - reconcile(ctx)        │
│ - handleTransition(ctx,t)│
│ - triggerFailover(ctx,id)│
│ + RegisterNode(id)      │
│ + UnregisterNode(id)    │
│ + ProcessHeartbeat(ctx) │
│ + GetNodeHealth(id)     │
│ + GetAllHealth()        │
│ + IsRunning() bool      │
│ + GetThresholds()       │
│ + SetThresholds(t) err  │
│ + TriggerFailover(ctx)  │
│ + GetStatus() map       │
└─────────────────────────┘
            │
            │ uses
            ▼
┌─────────────────────────┐         ┌─────────────────────────┐
│   HeartbeatProcessor    │         │     NodeHealth          │
├─────────────────────────┤         ├─────────────────────────┤
│ - mu: sync.RWMutex      │         │ - NodeID: string        │
│ - nodes: map[string]    │         │ - State: HealthState    │
│ - threshold: Thresholds │         │ - LastHeartbeat: time   │
│ - callback: func        │         │ - MissedHeartbeats: int │
├─────────────────────────┤         │ - FirstMissedAt: *time  │
│ + NewHBProc(t, cb)      │         │ - LastSequence: uint64  │
│ + RegisterNode(id)      │         │ - VMs: []string         │
│ + UnregisterNode(id)    │         │ - Resources: HostRes    │
│ + ProcessHeartbeat(ctx) │         │ - mu: sync.RWMutex      │
│ + CheckAll(now) []Trans │         ├─────────────────────────┤
│ + GetNodeHealth(id)     │         │ + UpdateHeartbeat(hb)   │
│ + GetAllNodeHealth()    │         │ - MissHeartbeat(now,..) │
│ + GetOfflineNodes()     │         │ + GetState() HealthState│
│ + GetSuspectNodes()     │         │ - getVMs() []string     │
│ + GetOnlineNodes()      │         │ + GetMissedCount() int  │
└─────────────────────────┘         └─────────────────────────┘

┌─────────────────────────┐
│     HealthState         │
├─────────────────────────┤
│ StateOnline (0)         │
│ StateSuspect (1)        │
│ StateOffline (2)        │
├─────────────────────────┤
│ + String() string       │
└─────────────────────────┘

┌─────────────────────────┐
│     HealthThresholds    │
├─────────────────────────┤
│ - HeartbeatInterval     │
│ - SuspectThreshold      │
│ - OfflineThreshold      │
├─────────────────────────┤
│ + DefaultThresholds()   │
│ + Validate() error      │
└─────────────────────────┘
```

### HA Orchestrator Subsystem

```
┌─────────────────────────┐         ┌─────────────────────────┐
│   Orchestrator          │         │   FailoverRecord        │
│   (interface)           │         ├─────────────────────────┤
├─────────────────────────┤         │ - ID: string            │
│ + HandleHostFailure(ctx)│         │ - NodeID: string        │
│ + RestartVMs(ctx, vms)  │         │ - State: string         │
│ + GetActiveFailovers()  │         │ - VMCount: int          │
│ + GetFailoverHistory(n) │         │ - VMRestarted: int      │
└─────────────────────────┘         │ - VMFailed: int         │
            ▲                       │ - StartedAt: time       │
            │ implements             │ - CompletedAt: *time    │
            │                        │ - Error: string         │
┌─────────────────────────┐         └─────────────────────────┘
│  DefaultOrchestrator    │
├─────────────────────────┤         ┌─────────────────────────┐
│ - mu: sync.Mutex        │         │     OrchestratorConfig  │
│ - fencer: Fencer        │         ├─────────────────────────┤
│ - scheduler: Scheduler  │         │ - Fencer                │
│ - vmProvider: VMProv    │         │ - Scheduler             │
│ - hostProvider: HostPrv │         │ - VMProvider            │
│ - vmRestarter: VMRestr  │         │ - HostProvider          │
│ - eventPublisher: EvPub │         │ - VMRestarter           │
│ - activeFailovers: map  │         │ - EventPublisher        │
│ - history: []FailoverRec│         │ - Policy                │
│ - maxHistory: int       │         │ - MaxHistory            │
│ - policy: Policy        │         └─────────────────────────┘
├─────────────────────────┤
│ + NewOrchestrator(cfg)  │
│ + HandleHostFailure(ctx)│
│ + RestartVMs(ctx,vms,h) │
│ + GetActiveFailovers()  │
│ + GetFailoverHistory(n) │
│ - completeFailover(rec) │
│ - failFailover(rec,err) │
│ - addToHistory(rec)     │
└─────────────────────────┘

┌─────────────────────────┐         ┌─────────────────────────┐
│    VMProvider           │         │     HostProvider        │
│    (interface)          │         │     (interface)         │
├─────────────────────────┤         ├─────────────────────────┤
│ + GetVMsByHost(ctx,id)  │         │ + GetHost(ctx,id)       │
│ + GetVM(ctx,id)         │         │ + ListHosts(ctx,status) │
│ + UpdateVMHost(ctx,vm,h)│         └─────────────────────────┘
│ + GetHAPolicy(ctx,id)   │
│ + SetHAPolicy(ctx,id,p) │         ┌─────────────────────────┐
│ + ListHAPolicies(ctx)   │         │     VMRestarter         │
└─────────────────────────┘         │     (interface)         │
                                    ├─────────────────────────┤
┌─────────────────────────┐         │ + StartVM(ctx,vm,target)│
│    EventPublisher       │         │ + GetStartStatus(ctx,id)│
│    (interface)          │         └─────────────────────────┘
├─────────────────────────┤
│ + Publish(ctx, details) │
└─────────────────────────┘
```

### HA Fencing Subsystem

```
┌─────────────────────────┐         ┌─────────────────────────┐
│       Fencer            │         │     FenceConfig         │
│       (interface)       │         ├─────────────────────────┤
├─────────────────────────┤         │ - Method: FenceMethod   │
│ + Fence(ctx, nodeID)    │         │ - Host: string          │
│ + GetPowerState(ctx,id) │         │ - Port: int             │
│ + GetMethod() FenceMeth │         │ - Username: string      │
└─────────────────────────┘         │ - Password: string      │
            ▲                       │ - SSHKeyPath: string    │
            │                        │ - RedfishPath: string   │
            │ implements              │ - Timeout: time         │
            │                        │ - MaxRetries: int       │
            │                        │ - RetryInterval: time   │
            │                        ├─────────────────────────┤
            │                        │ + DefaultFenceConfig()  │
            │                        │ + Validate() error      │
            │                        └─────────────────────────┘
            │
    ┌───────┴───────┬───────────────────┐
    │               │                   │
    ▼               ▼                   ▼
┌────────────────┐ ┌────────────────┐ ┌────────────────┐
│  IPMIFencer    │ │ RedfishFencer  │ │  SSHFencer     │
├────────────────┤ ├────────────────┤ ├────────────────┤
│ - config       │ │ - config       │ │ - config       │
├────────────────┤ ├────────────────┤ ├────────────────┤
│ + Fence(ctx)   │ │ + Fence(ctx)   │ │ + Fence(ctx)   │
│ + GetPowerState│ │ + GetPowerState│ │ + GetPowerState│
│ + GetMethod()  │ │ + GetMethod()  │ │ + GetMethod()  │
└────────────────┘ └────────────────┘ └────────────────┘
        │                   │                   │
        └───────────────────┼───────────────────┘
                            │
                            ▼
                ┌─────────────────────────┐
                │     MultiFencer         │
                ├─────────────────────────┤
                │ - mu: sync.Mutex        │
                │ - fencers: []Fencer     │
                │ - results: []FenceRes   │
                │ - registry: map         │
                ├─────────────────────────┤
                │ + NewMultiFencer(cfgs)  │
                │ + Fence(ctx, nodeID)    │
                │ + GetPowerState(ctx,id) │
                │ + GetMethod()           │
                │ + recordResult(r)       │
                │ + GetResults()          │
                └─────────────────────────┘
```

### HA Scheduler Subsystem

```
┌─────────────────────────┐         ┌─────────────────────────┐
│       Scheduler         │         │       Policy            │
│       (interface)       │         ├─────────────────────────┤
├─────────────────────────┤         │ - PolicyType: string    │
│ + SelectTarget(vm,hosts)│         │ - PreferSameNUMANode   │
│ + SelectTargets(vms,h)  │         │ - MaxOvercommitRatio   │
│ + ScoreHost(vm,host)    │         │ - LoadBalanceWeight    │
└─────────────────────────┘         │ - NUMAAffinityWeight   │
            ▲                       │ - AntiAffinityEnabled  │
            │ implements             ├─────────────────────────┤
            │                        │ + DefaultPolicy()       │
            │                        └─────────────────────────┘
            │
┌─────────────────────────┐         ┌─────────────────────────┐
│   DefaultScheduler      │         │       Host              │
├─────────────────────────┤         ├─────────────────────────┤
│ - mu: sync.Mutex        │         │ - ID: string            │
├─────────────────────────┤         │ - Name: string          │
│ + NewDefaultScheduler() │         │ - Status: string        │
│ + SelectTarget(vm,hosts)│         │ - CPUCount: int         │
│ + SelectTargets(vms,h)  │         │ - MemoryTotalBytes      │
│ + ScoreHost(vm,host)    │         │ - MemoryUsedBytes       │
│ - filterHosts(vm,hosts) │         │ - NUMANodes: []NUMANode │
└─────────────────────────┘         │ - VMCount: int          │
                                    │ - MaintenanceMode: bool │
┌─────────────────────────┐         ├─────────────────────────┤
│         VM              │         │ + CanFit(cpu,mem) bool │
├─────────────────────────┤         │ + AvailableMemory()     │
│ - ID: string            │         └─────────────────────────┘
│ - Name: string          │
│ - CPUs: int             │         ┌─────────────────────────┐
│ - MemoryBytes: int64    │         │     NUMANode            │
│ - NUMAPolicy: *string   │         ├─────────────────────────┤
│ - AntiAffinity: []str   │         │ - ID: int               │
│ - PreferredHost: *str   │         │ - CPUCores: []int       │
│ - Labels: map           │         │ - MemoryBytes: int64    │
└─────────────────────────┘         │ - FreeMemory: int64     │
                                    │ - PinnedCPUs: []int     │
                                    └─────────────────────────┘
```

### HA Policy Manager

```
┌─────────────────────────┐         ┌─────────────────────────┐
│    PolicyManager        │         │     HAPolicy            │
├─────────────────────────┤         ├─────────────────────────┤
│ - mu: sync.RWMutex      │         │ - VMID: string          │
│ - policies: map         │         │ - Mode: HAMode          │
│ - restartCounts: map    │         │ - Priority: int         │
│ - historyStore: Store   │         │ - AntiAffinity: []str   │
├─────────────────────────┤         │ - MaxRestarts: int      │
│ + NewPolicyManager(str) │         │ - RestartWindow: time   │
│ + GetPolicy(id)         │         │ - LastModified: time    │
│ + SetPolicy(id,p,actor) │         ├─────────────────────────┤
│ + DeletePolicy(id)      │         │ + DefaultHAPolicy(id)  │
│ + ListPolicies()        │         │ + Validate() error      │
│ + GetAutoRestartVMs(ids)│         │ + ShouldRestart() bool  │
│ + RecordRestart(id)     │         └─────────────────────────┘
│ + CanRestart(id) bool   │
│ + GetAntiAffinityGrps() │         ┌─────────────────────────┐
│ + LoadDefaults(ids)     │         │       HAMode            │
└─────────────────────────┘         ├─────────────────────────┤
                                    │ HAModeAuto ("auto")     │
┌─────────────────────────┐         │ HAModeManual ("manual") │
│  PolicyHistoryStore     │         │ HAModeNever ("never")   │
│  (interface)            │         │ HAModeMaxOne ("max-one")│
├─────────────────────────┤         ├─────────────────────────┤
│ + RecordPolicyChange()  │         │ + String() string       │
└─────────────────────────┘         └─────────────────────────┘
```

### HA Storage Backends

```
┌─────────────────────────┐
│    StorageBackend       │
│    (interface)          │
├─────────────────────────┤
│ + Name() string         │
│ + Init() error          │
│ + IsAvailable() bool    │
└─────────────────────────┘
            ▲
            │ implements
    ┌───────┴───────┬───────────────────┐
    │               │                   │
    ▼               ▼                   ▼
┌────────────────┐ ┌────────────────┐ ┌────────────────┐
│  NFSBackend    │ │CephRBDBackend  │ │ ISCSIBackend   │
├────────────────┤ ├────────────────┤ ├────────────────┤
│ - Server       │ │ - PoolName     │ │ - Portal       │
│ - ExportPath   │ │ - ImageName    │ │ - TargetIQN    │
│ - MountPoint   │ │ - UserName     │ │ - Initiator    │
├────────────────┤ ├────────────────┤ ├────────────────┤
│ + Mount()      │ │ + Map()        │ │ + Login()      │
│ + Unmount()    │ │ + Unmap()      │ │ + Logout()     │
└────────────────┘ └────────────────┘ └────────────────┘
            │               │                   │
            └───────────────┼───────────────────┘
                            │ managed by
                            ▼
                ┌─────────────────────────┐
                │    StorageManager       │
                ├─────────────────────────┤
                │ - Backends: []StorBack  │
                ├─────────────────────────┤
                │ + Init() error          │
                │ + AvailableBackends()   │
                └─────────────────────────┘
```

### API Server

```
┌─────────────────────────┐
│       Server            │
├─────────────────────────┤
│ - migrationService      │
│ - tracker: Tracker      │
│ - preflight: Checker    │
│ - vcenter: Client       │
│ - httpServer: *Server   │
├─────────────────────────┤
│ + NewServer(addr, ...)  │
│ + Start() error         │
│ + Shutdown(ctx) error   │
│ - handleMigrationImportOVF() │
│ - handleMigrationImportOVA() │
│ - handleMigrationJobs() │
│ - handleMigrationJobDetail() │
│ - handleMigrationDiscover() │
│ - handleMigrationImportvCenter() │
│ - handleMigrationVCJobs() │
│ - handleMigrationVCJobDetail() │
│ - handleMigrationVCJobProgress() │
│ - executeImport(jobID)  │
└─────────────────────────┘

┌─────────────────────────┐
│    MigrationService     │
├─────────────────────────┤
│ - mu: sync.Mutex        │
│ - jobs: map[string]     │
│ - hosts: []string       │
├─────────────────────────┤
│ + NewMigrationService() │
│ + CreateImportJob()     │
│ + GetImportJob(id)      │
│ + ListImportJobs()      │
│ + UpdateJobState()      │
│ + CancelImportJob()     │
│ + DeleteImportJob()     │
│ + RunPreflightChecks()  │
└─────────────────────────┘
```

### Migration Tracker

```
┌─────────────────────────┐         ┌─────────────────────────┐
│       Tracker           │         │         Job             │
├─────────────────────────┤         ├─────────────────────────┤
│ - mu: sync.RWMutex      │         │ - ID: string            │
│ - jobs: map[string]*Job │         │ - State: JobState       │
│ - order: []string       │         │ - Source: ClientConfig  │
├─────────────────────────┤         │ - VMStatuses: map       │
│ + NewTracker()          │         │ - VMIDs: []string       │
│ + CreateJob(id,cfg,ids) │         │ - TotalBytes: int64     │
│ + GetJob(id)            │         │ - CopiedBytes: int64    │
│ + ListJobs()            │         │ - Error: string         │
│ + ListJobsByState(s)    │         │ - CreatedAt: time       │
│ + UpdateJobState(id,s)  │         │ - UpdatedAt: time       │
│ + UpdateVMStatus(id,fn) │         │ - StartedAt: *time      │
│ + AddCopiedBytes(id,n)  │         │ - CompletedAt: *time    │
│ + SetTotalBytes(id,n)   │         ├─────────────────────────┤
│ + SetJobError(id,err)   │         │ + Progress() float64    │
│ + CancelJob(id)         │         │ + VMCount() int         │
│ + DeleteJob(id)         │         │ + CompletedVMs() int    │
│ + ActiveJobs() int      │         │ + FailedVMs() int       │
│ + GetProgress(id)       │         └─────────────────────────┘
└─────────────────────────┘
            │
            │ uses
            ▼
┌─────────────────────────┐         ┌─────────────────────────┐
│     JobExecutor         │         │    VMImportStatus       │
├─────────────────────────┤         ├─────────────────────────┤
│ - tracker: Tracker      │         │ - VMID: string          │
│ - vcenter: Client       │         │ - State: string         │
├─────────────────────────┤         │ - Progress: float64     │
│ + NewJobExecutor(t,c)   │         │ - BytesTotal: int64     │
│ + Run(ctx, jobID)       │         │ - BytesCopied: int64    │
│ - execute(ctx, jobID)   │         │ - StartedAt: *time      │
└─────────────────────────┘         │ - CompletedAt: *time    │
                                    │ - Error: string         │
┌─────────────────────────┐         │ - PreflightChecks: []   │
│       JobState          │         └─────────────────────────┘
├─────────────────────────┤
│ JobStateQueued          │
│ JobStateDiscovering     │
│ JobStatePreflight       │
│ JobStateImporting       │
│ JobStateValidating      │
│ JobStateCompleted       │
│ JobStateFailed          │
│ JobStateCancelled       │
├─────────────────────────┤
│ + IsValid() bool        │
│ + String() string       │
└─────────────────────────┘
```

### Preflight Checker

```
┌─────────────────────────┐         ┌─────────────────────────┐
│       Checker           │         │        Result           │
├─────────────────────────┤         ├─────────────────────────┤
│ - TargetCPUFeatures     │         │ - VMID: string          │
│ - TargetMaxCPUs         │         │ - VMName: string        │
│ - TargetMaxMemoryMB     │         │ - Passed: bool          │
│ - TargetMaxDiskGB       │         │ - Checks: []CheckDetail │
│ - NetworkMappings       │         └─────────────────────────┘
│ - SupportedGuestOS      │
├─────────────────────────┤         ┌─────────────────────────┐
│ + DefaultChecker()      │         │     CheckDetail         │
│ + CheckVM(vm) Result    │         ├─────────────────────────┤
│ + CheckAll(vms) []Res   │         │ - Name: string          │
│ - checkCPU(vm)          │         │ - Passed: bool          │
│ - checkMemory(vm)       │         │ - Severity: string      │
│ - checkDisk(vm)         │         │ - Message: string       │
│ - checkGuestOS(vm)      │         └─────────────────────────┘
│ - checkNetworks(vm)     │
│ - checkVMwareTools(vm)  │
│ - checkPowerState(vm)   │
│ - checkProvisioningType()│
└─────────────────────────┘
```

### vCenter Client

```
┌─────────────────────────┐         ┌─────────────────────────┐
│       Client            │         │    ClientConfig         │
├─────────────────────────┤         ├─────────────────────────┤
│ - mu: sync.Mutex        │         │ - Host: string          │
│ - host: string          │         │ - Port: int             │
│ - port: int             │         │ - Username: string      │
│ - username: string      │         │ - Password: string      │
│ - insecure: bool        │         │ - Insecure: bool        │
│ - connected: bool       │         └─────────────────────────┘
│ - sessionExpiry: time   │
├─────────────────────────┤         ┌─────────────────────────┐
│ + NewClient(cfg)        │         │    DiscoveryResult      │
│ + Connect(ctx) error    │         ├─────────────────────────┤
│ + Disconnect() error    │         │ - Datacenters: []DC     │
│ + IsConnected() bool    │         │ - RetrievedAt: time     │
│ + Discover(ctx) *Result │         └─────────────────────────┘
│ + GetVM(ctx,id) *VM     │
│ + GetGuestInfo(ctx,id)  │         ┌─────────────────────────┐
└─────────────────────────┘         │      Datacenter         │
                                    ├─────────────────────────┤
┌─────────────────────────┐         │ - ID: string            │
│          VM             │         │ - Name: string          │
├─────────────────────────┤         │ - VMs: []VM             │
│ - ID: string            │         │ - Hosts: []Host         │
│ - Name: string          │         │ - Cluster: string       │
│ - PowerState: string    │         └─────────────────────────┘
│ - GuestOS: string       │
│ - NumCPUs: int          │         ┌─────────────────────────┐
│ - MemoryMB: int64       │         │         Host            │
│ - DiskGB: int64         │         ├─────────────────────────┤
│ - VMwareTools: string   │         │ - ID: string            │
│ - IPAddress: string     │         │ - Name: string          │
│ - HostName: string      │         │ - Model: string         │
│ - Networks: []string    │         │ - CPUMHz: int64         │
│ - Datastores: []string  │         │ - CPUCores: int         │
│ - ProvisioningType: str │         │ - MemoryBytes: int64    │
└─────────────────────────┘         │ - Connection: string    │
                                    └─────────────────────────┘
```

### OVF Parser

```
┌─────────────────────────┐
│     ParsedOVFResult     │
├─────────────────────────┤
│ - VMs: []ParsedVM       │
│ - Disks: []ParsedDisk   │
│ - Networks: []string    │
│ - RawSize: uint64       │
│ - OVFVersion: string    │
└─────────────────────────┘

┌─────────────────────────┐         ┌─────────────────────────┐
│      ParsedVM           │         │     ParsedDisk          │
├─────────────────────────┤         ├─────────────────────────┤
│ - ID: string            │         │ - ID: string            │
│ - Name: string          │         │ - FileRef: string       │
│ - OperatingSystem: str  │         │ - SizeBytes: uint64     │
│ - OSType: string        │         │ - Format: string        │
│ - CPUs: int             │         └─────────────────────────┘
│ - MemoryMB: uint64      │
│ - DiskRefs: []string    │
│ - NetworkRefs: []string │
│ - VirtualSystemType: str│
│ - Labels: map           │
└─────────────────────────┘

┌─────────────────────────┐
│      OVFEnvelope        │
├─────────────────────────┤
│ - References            │
│ - DiskSection           │
│ - NetworkSection        │
│ - VirtualSystem         │
│ - VirtualSystemCollection│
└─────────────────────────┘
```

## Sequence Diagrams

### vCenter VM Import Flow

```
Operator          API Server         Tracker         Preflight        vCenter Client
   │                  │                │                │                  │
   │ POST /vcenter/   │                │                │                  │
   │ import           │                │                │                  │
   │─────────────────►│                │                │                  │
   │                  │                │                │                  │
   │                  │ NewClient(cfg) │                │                  │
   │                  │──────────────────────────────────────────────────►│
   │                  │◄──────────────────────────────────────────────────│
   │                  │                │                │                  │
   │                  │ Connect(ctx)   │                │                  │
   │                  │──────────────────────────────────────────────────►│
   │                  │◄──────────────────────────────────────────────────│
   │                  │                │                │                  │
   │                  │ Discover(ctx)  │                │                  │
   │                  │──────────────────────────────────────────────────►│
   │                  │◄──────────────────────────────────────────────────│
   │                  │                │                │                  │
   │                  │ CheckAll(vms)  │                │                  │
   │                  │────────────────►                │                  │
   │                  │◄────────────────│                │                  │
   │                  │                │                │                  │
   │                  │ CreateJob()    │                │                  │
   │                  │────────────────►│                │                  │
   │                  │◄────────────────│                │                  │
   │                  │                │                │                  │
   │                  │ Run(ctx, jobID)│                │                  │
   │                  │────────────────►│                │                  │
   │                  │                │                │                  │
   │  202 Accepted    │                │ UpdateJobState()                  │
   │◄─────────────────│                │ queued → importing               │
   │                  │                │                │                  │
   │                  │                │ (async copy/convert)              │
   │                  │                │                │                  │
   │                  │                │ UpdateJobState()                  │
   │                  │                │ completed                         │
   │                  │                │                │                  │
   │ GET /jobs/:id    │                │                │                  │
   │─────────────────►│                │                │                  │
   │                  │ GetJob(id)     │                │                  │
   │                  │────────────────►│                │                  │
   │                  │◄────────────────│                │                  │
   │◄─────────────────│                │                │                  │
   │  Job Details     │                │                │                  │
```

### HA Failover Sequence

```
Host Agent       Controller        Orchestrator       Fencer          Scheduler
   │                │                  │                │                │
   │ heartbeat      │                  │                │                │
   │───────────────►│                  │                │                │
   │                │                  │                │                │
   │ (stops)        │                  │                │                │
   │                │ reconcile()      │                │                │
   │                │ CheckAll(now)    │                │                │
   │                │ (detects offline)│                │                │
   │                │                  │                │                │
   │                │ HandleHostFailure(ctx, nodeID)     │                │
   │                │─────────────────►│                │                │
   │                │                  │                │                │
   │                │                  │ GetVMsByHost() │                │
   │                │                  │───────────────►│ (VMProvider)   │
   │                │                  │◄───────────────│                │
   │                │                  │                │                │
   │                │                  │ Fence(ctx, id) │                │
   │                │                  │───────────────►│                │
   │                │                  │◄───────────────│                │
   │                │                  │                │                │
   │                │                  │ ListHosts(online)               │
   │                │                  │────────────────────────────────►│
   │                │                  │◄────────────────────────────────│
   │                │                  │                │                │
   │                │                  │ SelectTargets(vms, hosts)       │
   │                │                  │────────────────────────────────►│
   │                │                  │◄────────────────────────────────│
   │                │                  │                │                │
   │                │                  │ RestartVMs(ctx, vms, target)    │
   │                │                  │────────────────────────────────►│
   │                │                  │◄────────────────────────────────│
   │                │                  │                │                │
   │                │                  │ completeFailover(record)         │
   │                │◄─────────────────│                │                │
   │                │                  │                │                │
```

## State Machines

### Migration Job State Machine

```
                    ┌──────────┐
                    │  QUEUED  │
                    └────┬─────┘
                         │ JobExecutor.Run()
                         ▼
                    ┌──────────┐
            ┌──────►│DISCOVERING│
            │       └────┬─────┘
            │            │ Connect + Discover success
            │            ▼
            │       ┌──────────┐
            │       │PREFLIGHT │
            │       └────┬─────┘
            │            │ Preflight checks pass
            │            ▼
            │       ┌──────────┐
            │       │IMPORTING │
            │       └────┬─────┘
            │            │ Copy + Convert complete
            │            ▼
            │       ┌──────────┐
            │       │VALIDATING│
            │       └────┬─────┘
            │            │ Validation pass
            │            ▼
            │       ┌──────────┐
            │       │COMPLETED │
            │       └──────────┘
            │
            │ (any state)
            ├──────────────────┐
            │                  │
            ▼                  ▼
       ┌──────────┐      ┌──────────┐
       │  FAILED  │      │CANCELLED │
       └──────────┘      └──────────┘
```

### VM Import Status State Machine

```
┌─────────┐     ┌──────────┐     ┌───────────┐     ┌─────────┐     ┌──────┐
│ PENDING │────►│ COPYING  │────►│ CONVERTING│────►│ BOOTING │────►│ DONE │
└─────────┘     └──────────┘     └───────────┘     └─────────┘     └──────┘
                     │                │                │
                     └────────────────┴────────────────┘
                                      │
                                      ▼
                                 ┌──────────┐
                                 │  FAILED  │
                                 └──────────┘
```

### Import Job State Machine (OVF Upload)

```
┌─────────┐     ┌───────────┐     ┌────────────┐     ┌───────────┐
│ PENDING │────►│ PREFLIGHT │────►│  PARSING   │────►│ IMPORTING │────► COMPLETED
└─────────┘     └─────┬─────┘     └────────────┘     └───────────┘
                      │                                       │
                      │ (checks fail)                         │
                      ▼                                       ▼
                ┌──────────┐                            ┌──────────┐
                │  FAILED  │                            │  FAILED  │
                └──────────┘                            └──────────┘
                      │                                       │
                      └───────────────┬───────────────────────┘
                                      │
                                      ▼
                                ┌──────────┐
                                │CANCELLED │
                                └──────────┘
```

### HA Node Health State Machine

```
                      heartbeat received
         ┌─────────────────────────────────────────────┐
         │                                             │
         ▼                                             │
   ┌──────────┐   missed heartbeats ≥ 3   ┌──────────┐│
   │  ONLINE  │ ─────────────────────────► │  SUSPECT ││
   │          │ ◄───────────────────────── │          ││
   └──────────┘   heartbeat received        └────┬─────┘│
                                                 │      │
                                      missed heartbeats │
                                        ≥ 5            │
                                                 ▼      │
                                           ┌──────────┐ │
                                           │ OFFLINE  │ │
                                           │          │ │
                                           └──────────┘ │
                                                 │      │
                                                 └──────┘
                                          heartbeat received
```

### HA Failover State Machine

```
┌───────────┐     ┌─────────┐     ┌────────────┐     ┌────────────┐     ┌──────────┐
│ DETECTING │────►│ FENCING │────►│ SCHEDULING │────►│ RESTARTING │────►│ COMPLETE │
└───────────┘     └────┬────┘     └────────────┘     └────────────┘     └──────────┘
                       │                                                  │
                       │ (non-fatal)                                      │
                       └─────────┐                                        │
                                 │                                        │
                                 └────────────────────────────────────────┘
                                              │
                                              ▼
                                        ┌──────────┐
                                        │  FAILED  │
                                        └──────────┘
```

## Key Interfaces

```go
// HA Controller dependencies
type Orchestrator interface {
    HandleHostFailure(ctx context.Context, nodeID string) error
    RestartVMs(ctx context.Context, vms []VM, target Host) error
    GetActiveFailovers() []FailoverRecord
    GetFailoverHistory(limit int) []FailoverRecord
}

type Fencer interface {
    Fence(ctx context.Context, nodeID string) error
    GetPowerState(ctx context.Context, nodeID string) (string, error)
    GetMethod() FenceMethod
}

type Scheduler interface {
    SelectTarget(vm VM, hosts []Host, policy Policy) (*Host, error)
    SelectTargets(vms []VM, hosts []Host, policy Policy) (map[string]*Host, error)
    ScoreHost(vm VM, host Host, policy Policy) float64
}

type VMProvider interface {
    GetVMsByHost(ctx context.Context, hostID string) ([]VM, error)
    GetVM(ctx context.Context, vmID string) (*VM, error)
    UpdateVMHost(ctx context.Context, vmID, newHostID string) error
    GetHAPolicy(ctx context.Context, vmID string) (*HAPolicy, error)
    SetHAPolicy(ctx context.Context, vmID string, policy HAPolicy) error
    ListHAPolicies(ctx context.Context) (map[string]HAPolicy, error)
}

type HostProvider interface {
    GetHost(ctx context.Context, hostID string) (*Host, error)
    ListHosts(ctx context.Context, status string) ([]Host, error)
}

// Storage backends
type StorageBackend interface {
    Name() string
    Init() error
    IsAvailable() bool
}
```

## Error Handling Strategy

| Error Type | Handling | Retry |
|------------|----------|-------|
| Heartbeat timeout | State transition to suspect/offline | Automatic |
| Fencing failure | Log warning, continue failover | 3 attempts per method |
| VM restart failure | Increment failed count, continue | None |
| vCenter connection | Fail migration job | None (operator retry) |
| Storage mount failure | Alert operator | None |
| OVF parse error | Fail import job | None |
| Preflight check fail | Block import, report details | None |
