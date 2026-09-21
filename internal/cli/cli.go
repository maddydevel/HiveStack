// Package cli implements the HiveStack CLI.
//
// The CLI provides a command-line interface for HiveStack Manager.
// It uses cobra for command structure and supports table, JSON, and YAML output.
//
// Usage:
//
//	hive [global flags] <command> [command flags]
//
// Global flags:
//
//	--server    Manager API server URL (default: http://localhost:8080)
//	--token     API token for authentication
//	--config    Path to config file (default: ~/.hive/config.yaml)
//	--output    Output format: table, json, yaml (default: table)
//	--quiet     Suppress non-essential output
//	--verbose   Enable verbose output
//
// Commands:
//
//	Auth:
//	  login         Log in to the Manager
//	  logout        Log out
//	  whoami        Show current user
//
//	VMs:
//	  vm list                List all VMs
//	  vm get                 Get a VM by ID
//	  vm create              Create a new VM
//	  vm start               Start a VM
//	  vm stop                Stop a VM
//	  vm restart             Restart a VM
//	  vm migrate             Migrate a VM to another host
//	  vm snapshot            Manage VM snapshots
//	  vm console             Get console URL for a VM
//	  vm stats               Get VM statistics
//	  vm delete              Delete a VM
//
//	Hosts:
//	  host list              List all hosts
//	  host get               Get a host by ID
//	  host status            Get host status
//	  host maintenance       Toggle maintenance mode
//
//	Storage:
//	  storage pool list      List storage pools
//	  storage pool get       Get a storage pool
//	  storage pool create    Create a storage pool
//	  storage pool delete    Delete a storage pool
//	  disk list              List disks
//	  disk get               Get a disk
//	  disk resize            Resize a disk
//
//	Networks:
//	  network list           List networks
//	  network get            Get a network
//	  network create         Create a network
//	  network delete          Delete a network
//
//	Backups:
//	  backup list            List backups
//	  backup create          Create a backup
//	  backup restore         Restore a backup
//	  backup cancel          Cancel a backup
//
//	Migration:
//	  migrate import         Import from VMware (vcenter, ovf, vmx)
//	  migrate list           List migration jobs
//
//	System:
//	  health                 Check Manager health
//	  version                Show version information
package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile   string
	serverURL string
	apiToken  string
	output    string
	quiet     bool
	verbose   bool
)

// Init initializes the CLI with global flags and config.
func Init() {
	cobra.EnableCommandSorting = false

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "http://localhost:8080", "Manager API server URL")
	rootCmd.PersistentFlags().StringVar(&apiToken, "token", "", "API token for authentication")
	rootCmd.PersistentFlags().StringVar(&output, "output", "", "output format: table, json, yaml")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")

	viper.BindPFlag("server", rootCmd.PersistentFlags().Lookup("server"))
	viper.BindPFlag("token", rootCmd.PersistentFlags().Lookup("token"))
}

// ─── Root command ───────────────────────────────────────────────────────────────

var rootCmd = &cobra.Command{
	Use:   "hive",
	Short: "HiveStack CLI — manage your virtualization platform",
	Long: `HiveStack CLI provides a command-line interface to manage the HiveStack
virtualization platform.

Usage:
  hive [global flags] <command> [command flags]

Global Flags:
  --server    Manager API server URL (default: http://localhost:8080)
  --token     API token for authentication
  --output    Output format: table, json, yaml (default: table)
  --quiet     Suppress non-essential output
  --verbose   Enable verbose output

Commands:
  auth       Authentication commands (login, logout, whoami)
  vm         VM lifecycle commands (list, get, create, start, stop, restart, migrate, snapshot, console, stats, delete)
  host       Host management commands (list, get, status, maintenance)
  storage    Storage management commands (pool list/get/create/delete, disk list/get/resize)
  network    Network management commands (list, get, create, delete)
  backup     Backup management commands (list, create, restore, cancel)
  migrate    Migration commands (import, list)
  health     Check Manager health
  version    Show version information

Examples:
  hive vm list --server http://localhost:8080
  hive vm create my-vm --cpus 4 --mem 8192 --disk 20
  hive host list --output json
  hive backup create --vm vm-123`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("HiveStack CLI v0.1.0 — use 'hive --help' for available commands")
		fmt.Println("Run 'hive <command> --help' for command-specific help")
		return nil
	},
}

// ─── Auth commands ───────────────────────────────────────────────────────────────

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to the Manager",
	Long: `Authenticates with the HiveStack Manager and saves the API token.

Example:
  hive auth login --email admin@example.com --password secret`,
	RunE: func(cmd *cobra.Command, args []string) error {
		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")
		if email == "" || password == "" {
			return fmt.Errorf("email and password required (use --email and --password)")
		}
		fmt.Println("Login: use 'hive config set-token' or login via API")
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out",
	Long:  `Clears the saved API token.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Token cleared (placeholder — API integration pending)")
		return nil
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current user",
	Long:  `Shows the current authenticated user.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Whoami: use 'hive user me' or API integration pending")
		return nil
	},
}

// ─── VM commands ────────────────────────────────────────────────────────────────

var vmListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all VMs",
	Long: `Lists all virtual machines in the current tenant.

Example:
  hive vm list --output json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("VM list: use 'hive vm list' — API integration pending")
		return nil
	},
}

var vmGetCmd = &cobra.Command{
	Use:   "get <vm-id>",
	Short: "Get a VM by ID",
	Long: `Shows detailed information about a specific VM.

Example:
  hive vm get vm-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("VM '%s': API integration pending\n", args[0])
		return nil
	},
}

var vmCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new VM",
	Long: `Creates a new virtual machine.

Example:
  hive vm create my-vm --cpus 4 --mem 8192 --disk 20 --os linux

Flags:
  --cpus      Number of CPUs (default: 1)
  --mem       Memory in MB (default: 1024)
  --disk      Disk size in GB (default: 10)
  --os        OS type (default: linux)
  --template  Template ID to clone from`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cpus, _ := cmd.Flags().GetInt("cpus")
		mem, _ := cmd.Flags().GetInt("mem")
		disk, _ := cmd.Flags().GetInt("disk")
		osType, _ := cmd.Flags().GetString("os")
		fmt.Printf("Creating VM '%s': cpus=%d, mem=%dMB, disk=%dGB, os=%s\n", name, cpus, mem, disk, osType)
		fmt.Println("VM creation: API integration pending")
		return nil
	},
}

var vmStartCmd = &cobra.Command{
	Use:   "start <vm-id>",
	Short: "Start a VM",
	Long: `Starts a stopped virtual machine.

Example:
  hive vm start vm-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Starting VM '%s'... (API integration pending)\n", args[0])
		return nil
	},
}

var vmStopCmd = &cobra.Command{
	Use:   "stop <vm-id>",
	Short: "Stop a VM",
	Long: `Stops a running virtual machine.

Example:
  hive vm stop vm-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Stopping VM '%s'... (API integration pending)\n", args[0])
		return nil
	},
}

var vmRestartCmd = &cobra.Command{
	Use:   "restart <vm-id>",
	Short: "Restart a VM",
	Long: `Restarts a virtual machine.

Example:
  hive vm restart vm-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Restarting VM '%s'... (API integration pending)\n", args[0])
		return nil
	},
}

var vmMigrateCmd = &cobra.Command{
	Use:   "migrate <vm-id> --target <host-id>",
	Short: "Migrate a VM to another host",
	Long: `Migrates a running VM to another host using live migration.

Example:
  hive vm migrate vm-abc123 --target host-xyz789`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target, _ := cmd.Flags().GetString("target")
		if target == "" {
			return fmt.Errorf("--target required (host ID to migrate to)")
		}
		fmt.Printf("Migrating VM '%s' to host '%s'... (API integration pending)\n", args[0], target)
		return nil
	},
}

var vmSnapshotCmd = &cobra.Command{
	Use:   "snapshot <vm-id> [flags]",
	Short: "Manage VM snapshots",
	Long: `Manage snapshots for a VM.

Example:
  hive vm snapshot vm-abc123 --action create --name pre-update
  hive vm snapshot vm-abc123 --action list
  hive vm snapshot vm-abc123 --action delete --name pre-update`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		action, _ := cmd.Flags().GetString("action")
		name, _ := cmd.Flags().GetString("name")
		fmt.Printf("Snapshot action='%s' name='%s' vm='%s' (API integration pending)\n", action, name, args[0])
		return nil
	},
}

var vmConsoleCmd = &cobra.Command{
	Use:   "console <vm-id>",
	Short: "Get console URL for a VM",
	Long: `Returns the VNC/console URL for accessing a VM.

Example:
  hive vm console vm-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Console URL for VM '%s': API integration pending\n", args[0])
		return nil
	},
}

var vmStatsCmd = &cobra.Command{
	Use:   "stats <vm-id>",
	Short: "Get VM statistics",
	Long: `Returns real-time CPU, memory, disk, and network statistics for a VM.

Example:
  hive vm stats vm-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Stats for VM '%s': API integration pending\n", args[0])
		return nil
	},
}

var vmDeleteCmd = &cobra.Command{
	Use:   "delete <vm-id>",
	Short: "Delete a VM",
	Long: `Deletes a virtual machine and its associated resources.

WARNING: This operation is irreversible.

Example:
  hive vm delete vm-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Deleting VM '%s'... (API integration pending)\n", args[0])
		return nil
	},
}

// ─── Host commands ──────────────────────────────────────────────────────────────

var hostListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all hosts",
	Long: `Lists all registered HiveStack hosts.

Example:
  hive host list --output json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Host list: use 'hive host list' — API integration pending")
		return nil
	},
}

var hostGetCmd = &cobra.Command{
	Use:   "get <host-id>",
	Short: "Get a host by ID",
	Long: `Shows detailed information about a specific host.

Example:
  hive host get host-xyz789`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Host '%s': API integration pending\n", args[0])
		return nil
	},
}

// ─── Storage commands ───────────────────────────────────────────────────────────

var storagePoolListCmd = &cobra.Command{
	Use:   "list",
	Short: "List storage pools",
	Long: `Lists all storage pools.

Example:
  hive storage pool list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Storage pool list: API integration pending")
		return nil
	},
}

var storagePoolGetCmd = &cobra.Command{
	Use:   "get <pool-id>",
	Short: "Get a storage pool",
	Long: `Shows detailed information about a storage pool.

Example:
  hive storage pool get pool-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Storage pool '%s': API integration pending\n", args[0])
		return nil
	},
}

var storagePoolCreateCmd = &cobra.Command{
	Use:   "create <name> --type <type> --path <path>",
	Short: "Create a storage pool",
	Long: `Creates a new storage pool.

Example:
  hive storage pool create my-pool --type dir --path /var/lib/hivestack/storage

Flags:
  --type  Storage pool type (dir, lvm, ceph, etc.)
  --path  Path to the storage backend`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		poolType, _ := cmd.Flags().GetString("type")
		poolPath, _ := cmd.Flags().GetString("path")
		fmt.Printf("Creating storage pool '%s': type=%s, path=%s\n", args[0], poolType, poolPath)
		fmt.Println("Storage pool creation: API integration pending")
		return nil
	},
}

var storagePoolDeleteCmd = &cobra.Command{
	Use:   "delete <pool-id>",
	Short: "Delete a storage pool",
	Long: `Deletes a storage pool.

Warning: This may delete all volumes in the pool.

Example:
  hive storage pool delete pool-xyz789`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Deleting storage pool '%s'... (API integration pending)\n", args[0])
		return nil
	},
}

var diskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List disks",
	Long: `Lists all storage volumes/disks.

Example:
  hive disk list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Disk list: API integration pending")
		return nil
	},
}

var diskGetCmd = &cobra.Command{
	Use:   "get <disk-id>",
	Short: "Get a disk",
	Long: `Shows detailed information about a disk/volume.

Example:
  hive disk get vol-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Disk '%s': API integration pending\n", args[0])
		return nil
	},
}

var diskResizeCmd = &cobra.Command{
	Use:   "resize <disk-id> <size>",
	Short: "Resize a disk",
	Long: `Resizes a storage volume to the specified size in bytes.

Example:
  hive disk resize vol-abc123 21474836480`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		size, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid size: %s (must be a number in bytes)", args[1])
		}
		fmt.Printf("Resizing disk '%s' to %d bytes... (API integration pending)\n", args[0], size)
		return nil
	},
}

// ─── Network commands ───────────────────────────────────────────────────────────

var networkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List networks",
	Long: `Lists all virtual networks.

Example:
  hive network list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Network list: API integration pending")
		return nil
	},
}

var networkGetCmd = &cobra.Command{
	Use:   "get <network-id>",
	Short: "Get a network",
	Long: `Shows detailed information about a virtual network.

Example:
  hive network get net-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Network '%s': API integration pending\n", args[0])
		return nil
	},
}

var networkCreateCmd = &cobra.Command{
	Use:   "create <name> --type <type>",
	Short: "Create a network",
	Long: `Creates a new virtual network.

Example:
  hive network create my-net --type bridge

Flags:
  --type  Network type (bridge, nat, routed, etc.)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		netType, _ := cmd.Flags().GetString("type")
		fmt.Printf("Creating network '%s': type=%s (API integration pending)\n", args[0], netType)
		return nil
	},
}

var networkDeleteCmd = &cobra.Command{
	Use:   "delete <network-id>",
	Short: "Delete a network",
	Long: `Deletes a virtual network.

Example:
  hive network delete net-xyz789`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Deleting network '%s'... (API integration pending)\n", args[0])
		return nil
	},
}

// ─── Backup commands ────────────────────────────────────────────────────────────

var backupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List backups",
	Long: `Lists all backups.

Example:
  hive backup list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Backup list: API integration pending")
		return nil
	},
}

var backupCreateCmd = &cobra.Command{
	Use:   "create --vm <vm-id>",
	Short: "Create a backup",
	Long: `Creates a backup of a VM.

Example:
  hive backup create --vm vm-abc123`,
	RunE: func(cmd *cobra.Command, args []string) error {
		vmID, _ := cmd.Flags().GetString("vm")
		if vmID == "" {
			return fmt.Errorf("--vm required (VM ID to backup)")
		}
		fmt.Printf("Creating backup for VM '%s'... (API integration pending)\n", vmID)
		return nil
	},
}

var backupRestoreCmd = &cobra.Command{
	Use:   "restore <backup-id>",
	Short: "Restore a backup",
	Long: `Restores a VM from a backup.

Example:
  hive backup restore backup-xyz789`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Restoring backup '%s'... (API integration pending)\n", args[0])
		return nil
	},
}

var backupCancelCmd = &cobra.Command{
	Use:   "cancel <backup-id>",
	Short: "Cancel a backup",
	Long: `Cancels an in-progress backup operation.

Example:
  hive backup cancel backup-xyz789`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Cancelling backup '%s'... (API integration pending)\n", args[0])
		return nil
	},
}

// ─── Migration commands ─────────────────────────────────────────────────────────

var migrateImportCmd = &cobra.Command{
	Use:   "import <source> --type <vcenter|ovf|vmx>",
	Short: "Import from VMware",
	Long: `Import VMs from VMware sources.

Types:
  vcenter  - Import from vCenter (requires --vcenter-url, --username, --password)
  ovf      - Import OVF/OVA file (requires --file)
  vmx      - Import VMX file (requires --file)

Examples:
  hive migrate import --type vmx --file /path/to/vm.vmx
  hive migrate import --type vcenter --vcenter-url https://vc.example.com --username admin --password secret`,
	RunE: func(cmd *cobra.Command, args []string) error {
		importType, _ := cmd.Flags().GetString("type")
		file, _ := cmd.Flags().GetString("file")
		vcURL, _ := cmd.Flags().GetString("vcenter-url")
		vcUser, _ := cmd.Flags().GetString("username")

		switch importType {
		case "vmx":
			if file == "" {
				return fmt.Errorf("--file required for vmx import")
			}
			fmt.Printf("Importing VMX file '%s'... (VMX parser available, API integration pending)\n", file)
		case "vcenter":
			if vcURL == "" || vcUser == "" {
				return fmt.Errorf("--vcenter-url and --username required for vcenter import")
			}
			fmt.Printf("Importing from vCenter '%s'... (API integration pending)\n", vcURL)
		case "ovf":
			if file == "" {
				return fmt.Errorf("--file required for ovf import")
			}
			fmt.Printf("Importing OVF file '%s'... (API integration pending)\n", file)
		default:
			return fmt.Errorf("unknown import type: %s (use vcenter, ovf, or vmx)", importType)
		}
		return nil
	},
}

var migrateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List migration jobs",
	Long: `Lists all migration jobs and their status.

Example:
  hive migrate list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Migration list: API integration pending")
		return nil
	},
}

// ─── System commands ────────────────────────────────────────────────────────────

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check Manager health",
	Long: `Checks the health status of the HiveStack Manager.

Example:
  hive health --server http://localhost:8080`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Checking health at %s... (API integration pending)\n", serverURL)
		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Displays the HiveStack CLI version and build information.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("HiveStack CLI v0.1.0 (development build)")
		return nil
	},
}

// ─── Command registration ──────────────────────────────────────────────────────

func addAuthCommands() {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
		Long: `Manage authentication with the HiveStack Manager.

Commands:
  login   Log in to the Manager
  logout  Log out
  whoami  Show current user

Example:
  hive auth login --email admin@example.com --password secret`,
	}

	authCmd.AddCommand(loginCmd, logoutCmd, whoamiCmd)
	rootCmd.AddCommand(authCmd)
}

func addVmCommands() {
	vmCmd := &cobra.Command{
		Use:   "vm",
		Short: "VM lifecycle commands",
		Long: `Manage virtual machines.

Commands:
  list        List all VMs
  get         Get a VM by ID
  create      Create a new VM
  start       Start a VM
  stop        Stop a VM
  restart     Restart a VM
  migrate     Migrate a VM to another host
  snapshot    Manage VM snapshots
  console     Get console URL for a VM
  stats       Get VM statistics
  delete      Delete a VM

VM create flags:
  --cpus      Number of CPUs (default: 1)
  --mem       Memory in MB (default: 1024)
  --disk      Disk size in GB (default: 10)
  --os        OS type (default: linux)
  --template  Template ID to clone from

VM migrate flags:
  --target    Target host ID (required)

VM snapshot flags:
  --action    Action: create, list, delete (default: create)
  --name      Snapshot name (for create)

Examples:
  hive vm list
  hive vm create my-vm --cpus 4 --mem 8192 --disk 20
  hive vm start vm-abc123
  hive vm migrate vm-abc123 --target host-xyz789`,
	}

	vmCmd.AddCommand(vmListCmd, vmGetCmd, vmCreateCmd, vmStartCmd, vmStopCmd, vmRestartCmd, vmMigrateCmd, vmSnapshotCmd, vmConsoleCmd, vmStatsCmd, vmDeleteCmd)

	// VM create flags
	vmCreateCmd.Flags().IntP("cpus", "c", 1, "Number of CPUs")
	vmCreateCmd.Flags().IntP("mem", "m", 1024, "Memory in MB")
	vmCreateCmd.Flags().IntP("disk", "d", 10, "Disk size in GB")
	vmCreateCmd.Flags().StringP("os", "o", "linux", "OS type")
	vmCreateCmd.Flags().StringP("template", "t", "", "Template ID")

	// VM migrate flags
	vmMigrateCmd.Flags().StringP("target", "t", "", "Target host ID (required)")

	// VM snapshot flags
	vmSnapshotCmd.Flags().StringP("action", "a", "create", "Action: create, list, delete")
	vmSnapshotCmd.Flags().StringP("name", "n", "", "Snapshot name (for create)")

	rootCmd.AddCommand(vmCmd)
}

func addHostCommands() {
	hostCmd := &cobra.Command{
		Use:   "host",
		Short: "Host management commands",
		Long: `Manage HiveStack hosts (physical nodes running KVM).

Commands:
  list        List all hosts
  get         Get a host by ID
  status      Get host status
  maintenance Toggle maintenance mode

Examples:
  hive host list
  hive host get host-xyz789`,
	}

	hostCmd.AddCommand(hostListCmd, hostGetCmd)
	rootCmd.AddCommand(hostCmd)
}

func addStorageCommands() {
	storageCmd := &cobra.Command{
		Use:   "storage",
		Short: "Storage management commands",
		Long: `Manage storage pools and disks.

Commands:
  pool        Storage pool commands (list, get, create, delete)
  disk        Disk/volume commands (list, get, resize)

Examples:
  hive storage pool list
  hive storage pool create my-pool --type dir --path /var/lib/hivestack/storage
  hive disk list
  hive disk resize vol-abc123 21474836480`,
	}

	diskCmd := &cobra.Command{
		Use:   "disk",
		Short: "Disk/volume commands",
		Long: `Manage storage volumes/disks.

Commands:
  list        List all disks
  get         Get a disk by ID
  resize      Resize a disk

Examples:
  hive disk list
  hive disk get vol-abc123
  hive disk resize vol-abc123 21474836480`,
	}

	diskCmd.AddCommand(diskListCmd, diskGetCmd, diskResizeCmd)
	storageCmd.AddCommand(diskCmd)

	poolCmd := &cobra.Command{
		Use:   "pool",
		Short: "Storage pool commands",
		Long: `Manage storage pools.

Commands:
  list        List storage pools
  get         Get a storage pool
  create      Create a storage pool
  delete      Delete a storage pool

Storage pool create flags:
  --type  Storage pool type (dir, lvm, ceph, etc.)
  --path  Path to the storage backend

Examples:
  hive storage pool list
  hive storage pool create my-pool --type dir --path /var/lib/hivestack/storage`,
	}

	poolCmd.AddCommand(storagePoolListCmd, storagePoolGetCmd, storagePoolCreateCmd, storagePoolDeleteCmd)
	storageCmd.AddCommand(poolCmd)

	rootCmd.AddCommand(storageCmd)
}

func addNetworkCommands() {
	networkCmd := &cobra.Command{
		Use:   "network",
		Short: "Network management commands",
		Long: `Manage virtual networks.

Commands:
  list        List networks
  get         Get a network
  create      Create a network
  delete      Delete a network

Network create flags:
  --type  Network type (bridge, nat, routed, etc.)

Examples:
  hive network list
  hive network create my-net --type bridge`,
	}

	networkCmd.AddCommand(networkListCmd, networkGetCmd, networkCreateCmd, networkDeleteCmd)
	rootCmd.AddCommand(networkCmd)
}

func addBackupCommands() {
	backupCmd := &cobra.Command{
		Use:   "backup",
		Short: "Backup management commands",
		Long: `Manage VM backups.

Commands:
  list        List backups
  create      Create a backup
  restore     Restore a backup
  cancel      Cancel a backup

Backup create flags:
  --vm  VM ID to backup (required)

Examples:
  hive backup list
  hive backup create --vm vm-abc123
  hive backup restore backup-xyz789`,
	}

	backupCmd.AddCommand(backupListCmd, backupCreateCmd, backupRestoreCmd, backupCancelCmd)
	rootCmd.AddCommand(backupCmd)
}

func addMigrationCommands() {
	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migration commands",
		Long: `Import VMs from VMware sources and manage migration jobs.

Commands:
  import      Import from VMware (vcenter, ovf, vmx)
  list        List migration jobs

Migration import flags:
  --type       Import type: vcenter, ovf, vmx (required)
  --file       Source file path (for vmx, ovf)
  --vcenter-url vCenter URL (for vcenter)
  --username   vCenter username (for vcenter)
  --password   vCenter password (for vcenter)

Examples:
  hive migrate import --type vmx --file /path/to/vm.vmx
  hive migrate list`,
	}

	migrateCmd.AddCommand(migrateImportCmd, migrateListCmd)

	migrateImportCmd.Flags().StringP("type", "t", "", "Import type: vcenter, ovf, vmx (required)")
	migrateImportCmd.Flags().StringP("file", "f", "", "Source file path")
	migrateImportCmd.Flags().StringP("vcenter-url", "u", "", "vCenter URL")
	migrateImportCmd.Flags().StringP("username", "U", "", "vCenter username")
	migrateImportCmd.Flags().StringP("password", "p", "", "vCenter password")

	rootCmd.AddCommand(migrateCmd)
}

func addSystemCommands() {
	systemCmd := &cobra.Command{
		Use:   "system",
		Short: "System commands",
		Long: `System-level commands for HiveStack.

Commands:
  health      Check Manager health
  version     Show version information`,
	}

	healthCmd.Flags().StringP("server", "s", serverURL, "Manager API server URL")

	systemCmd.AddCommand(healthCmd, versionCmd)
	rootCmd.AddCommand(systemCmd)
}

// Execute adds all commands to the root command and executes it.
func Execute() error {
	addAuthCommands()
	addVmCommands()
	addHostCommands()
	addStorageCommands()
	addNetworkCommands()
	addBackupCommands()
	addMigrationCommands()
	addSystemCommands()
	rootCmd.Execute()
	return nil
}
