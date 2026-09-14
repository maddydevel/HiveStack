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
    cfgFile     string
    serverURL   string
    apiToken    string
    output      string
    quiet       bool
    verbose     bool
)

// Init initializes the CLI with global flags and config.
func Init() {
    // Set up Viper for config file
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath("$HOME/.hive")
    viper.AddConfigPath(".")

    if cfgFile != "" {
        viper.SetConfigFile(cfgFile)
    }

    viper.BindPFlag("server", rootCmd.PersistentFlags().Lookup("server"))
    viper.BindPFlag("token", rootCmd.PersistentFlags().Lookup("token"))
    viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
    viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
    viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

    viper.AutomaticEnv()
}

// Run runs the CLI.
func Run(args []string) error {
    Init()
    return rootCmd.Execute()
}

// rootCmd is the root command.
var rootCmd = &cobra.Command{
    Use:   "hive",
    Short: "HiveStack CLI — manage KVM virtualization",
    Long: `HiveStack CLI — command-line interface for HiveStack.

HiveStack is a KVM-based virtualization management platform on SUSE SLES 15 SP7.
This CLI connects to the HiveStack Manager REST API.

Examples:
  hive vm list
  hive vm create my-vm --cpus 4 --mem 8192
  hive vm start my-vm
  hive host list
  hive storage pool list
  hive backup create --vm my-vm`,
    PersistentPreRun: func(cmd *cobra.Command, args []string) {
        if serverURL == "" {
            serverURL = viper.GetString("server")
            if serverURL == "" {
                serverURL = "http://localhost:8080"
            }
        }
        if apiToken == "" {
            apiToken = viper.GetString("token")
        }
        if output == "" {
            output = viper.GetString("output")
            if output == "" {
                output = "table"
            }
        }
        quiet = viper.GetBool("quiet")
        verbose = viper.GetBool("verbose")
    },
}

func init() {
    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
    rootCmd.PersistentFlags().StringVar(&serverURL, "server", "", "Manager API server URL")
    rootCmd.PersistentFlags().StringVar(&apiToken, "token", "", "API token for authentication")
    rootCmd.PersistentFlags().StringVar(&output, "output", "", "output format: table, json, yaml")
    rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-essential output")
    rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")

    // Mark config as required for commands that need it (actually optional, just a placeholder)
}

// ─── Auth commands ───────────────────────────────────────────────────────────

var loginCmd = &cobra.Command{
    Use:   "login",
    Short: "Log in to the Manager",
    Long:  `Authenticates with the HiveStack Manager and saves the API token.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var logoutCmd = &cobra.Command{
    Use:   "logout",
    Short: "Log out",
    Long:  `Clears the saved API token.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var whoamiCmd = &cobra.Command{
    Use:   "whoami",
    Short: "Show current user",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

// ─── VM commands ─────────────────────────────────────────────────────────────

var vmListCmd = &cobra.Command{
    Use:   "list",
    Short: "List all VMs",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var vmGetCmd = &cobra.Command{
    Use:   "get <vm-id>",
    Short: "Get a VM by ID",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("VM '%s': (not implemented)\n", args[0])
        return nil
    },
}

var vmCreateCmd = &cobra.Command{
    Use:   "create <name>",
    Short: "Create a new VM",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        name := args[0]
        cpus, _ := cmd.Flags().GetInt("cpus")
        mem, _ := cmd.Flags().GetInt("mem")
        fmt.Printf("Creating VM '%s' — cpus=%d, mem=%dMB\n", name, cpus, mem)
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var vmStartCmd = &cobra.Command{
    Use:   "start <vm-id>",
    Short: "Start a VM",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("Starting VM '%s'...\n", args[0])
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var vmStopCmd = &cobra.Command{
    Use:   "stop <vm-id>",
    Short: "Stop a VM",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("Stopping VM '%s'...\n", args[0])
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var vmRestartCmd = &cobra.Command{
    Use:   "restart <vm-id>",
    Short: "Restart a VM",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("Restarting VM '%s'...\n", args[0])
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var vmMigrateCmd = &cobra.Command{
    Use:   "migrate <vm-id> --target <host-id>",
    Short: "Migrate a VM to another host",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        target, _ := cmd.Flags().GetString("target")
        fmt.Printf("Migrating VM '%s' to host '%s'...\n", args[0], target)
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var vmSnapshotCmd = &cobra.Command{
    Use:   "snapshot <vm-id> [flags]",
    Short: "Manage VM snapshots",
    Args:  cobra.MinimumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        action, _ := cmd.Flags().GetString("action")
        name, _ := cmd.Flags().GetString("name")
        fmt.Printf("Snapshot action='%s' name='%s' vm='%s'\n", action, name, args[0])
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var vmConsoleCmd = &cobra.Command{
    Use:   "console <vm-id>",
    Short: "Get console URL for a VM",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("Console URL for VM '%s': (not implemented)\n", args[0])
        return nil
    },
}

var vmStatsCmd = &cobra.Command{
    Use:   "stats <vm-id>",
    Short: "Get VM statistics",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("Stats for VM '%s': (not implemented)\n", args[0])
        return nil
    },
}

var vmDeleteCmd = &cobra.Command{
    Use:   "delete <vm-id>",
    Short: "Delete a VM",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("Deleting VM '%s'...\n", args[0])
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

// ─── Host commands ────────────────────────────────────────────────────────────

var hostListCmd = &cobra.Command{
    Use:   "list",
    Short: "List all hosts",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var hostGetCmd = &cobra.Command{
    Use:   "get <host-id>",
    Short: "Get a host by ID",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("Host '%s': (not implemented)\n", args[0])
        return nil
    },
}

// ─── Storage commands ─────────────────────────────────────────────────────────

var storagePoolListCmd = &cobra.Command{
    Use:   "list",
    Short: "List storage pools",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var storagePoolGetCmd = &cobra.Command{
    Use:   "get <pool-id>",
    Short: "Get a storage pool",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("Storage pool '%s': (not implemented)\n", args[0])
        return nil
    },
}

var storagePoolCreateCmd = &cobra.Command{
    Use:   "create <name> --type <type> --path <path>",
    Short: "Create a storage pool",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        poolType, _ := cmd.Flags().GetString("type")
        poolPath, _ := cmd.Flags().GetString("path")
        fmt.Printf("Creating storage pool '%s' — type=%s path=%s\n", args[0], poolType, poolPath)
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var storagePoolDeleteCmd = &cobra.Command{
    Use:   "delete <pool-id>",
    Short: "Delete a storage pool",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("Deleting storage pool '%s'...\n", args[0])
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var diskListCmd = &cobra.Command{
    Use:   "list",
    Short: "List disks",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var diskGetCmd = &cobra.Command{
    Use:   "get <disk-id>",
    Short: "Get a disk",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("Disk '%s': (not implemented)\n", args[0])
        return nil
    },
}

var diskResizeCmd = &cobra.Command{
    Use:   "resize <disk-id> <size>",
    Short: "Resize a disk",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        size, err := strconv.ParseUint(args[1], 10, 64)
        if err != nil {
            return fmt.Errorf("invalid size: %s", args[1])
        }
        fmt.Printf("Resizing disk '%s' to %d bytes...\n", args[0], size)
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

// ─── Network commands ─────────────────────────────────────────────────────────

var networkListCmd = &cobra.Command{
    Use:   "list",
    Short: "List networks",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var networkGetCmd = &cobra.Command{
    Use:   "get <network-id>",
    Short: "Get a network",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("Network '%s': (not implemented)\n", args[0])
        return nil
    },
}

var networkCreateCmd = &cobra.Command{
    Use:   "create <name> --type <type>",
    Short: "Create a network",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        netType, _ := cmd.Flags().GetString("type")
        fmt.Printf("Creating network '%s' — type=%s\n", args[0], netType)
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var networkDeleteCmd = &cobra.Command{
    Use:   "delete <network-id>",
    Short: "Delete a network",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("Deleting network '%s'...\n", args[0])
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

// ─── Backup commands ──────────────────────────────────────────────────────────

var backupListCmd = &cobra.Command{
    Use:   "list",
    Short: "List backups",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var backupCreateCmd = &cobra.Command{
    Use:   "create --vm <vm-id>",
    Short: "Create a backup",
    RunE: func(cmd *cobra.Command, args []string) error {
        vmID, _ := cmd.Flags().GetString("vm")
        fmt.Printf("Creating backup for VM '%s'...\n", vmID)
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var backupRestoreCmd = &cobra.Command{
    Use:   "restore <backup-id>",
    Short: "Restore a backup",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("Restoring backup '%s'...\n", args[0])
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var backupCancelCmd = &cobra.Command{
    Use:   "cancel <backup-id>",
    Short: "Cancel a backup",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("Cancelling backup '%s'...\n", args[0])
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

// ─── Migration commands ───────────────────────────────────────────────────────

var migrateImportCmd = &cobra.Command{
    Use:   "import <source> --type <vcenter|ovf|vmx>",
    Short: "Import from VMware",
    Long:  `Import VMs from VMware sources.

Types:
  vcenter  - Import from vCenter (requires --vcenter-url, --username, --password)
  ovf      - Import OVF/OVA file (requires --file)
  vmx      - Import VMX file (requires --file)`,
    RunE: func(cmd *cobra.Command, args []string) error {
        importType, _ := cmd.Flags().GetString("type")
        file, _ := cmd.Flags().GetString("file")
        vcURL, _ := cmd.Flags().GetString("vcenter-url")
        vcUser, _ := cmd.Flags().GetString("username")
        fmt.Printf("Importing from %s — file=%s vcenter=%s user=%s\n", importType, file, vcURL, vcUser)
        fmt.Println("(not implemented — VMware migration tools pending)")
        return nil
    },
}

var migrateListCmd = &cobra.Command{
    Use:   "list",
    Short: "List migration jobs",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

// ─── System commands ──────────────────────────────────────────────────────────

var healthCmd = &cobra.Command{
    Use:   "health",
    Short: "Check Manager health",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Printf("Checking health at %s...\n", serverURL)
        fmt.Println("  (not implemented — API integration pending)")
        return nil
    },
}

var versionCmd = &cobra.Command{
    Use:   "version",
    Short: "Show version information",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Println("HiveStack CLI v0.1.0 (development build)")
        return nil
    },
}

// ─── Command registration ─────────────────────────────────────────────────────

func addAuthCommands() {
    authCmd := &cobra.Command{
        Use:   "auth",
        Short: "Authentication commands",
        Long:  `Manage authentication with the HiveStack Manager.

Commands:
  login   Log in to the Manager
  logout  Log out
  whoami  Show current user`,
    }

    authCmd.AddCommand(loginCmd, logoutCmd, whoamiCmd)
    rootCmd.AddCommand(authCmd)
}

func addVmCommands() {
    vmCmd := &cobra.Command{
        Use:   "vm",
        Short: "VM lifecycle commands",
        Long:  `Manage virtual machines.

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
  --template  Template ID to clone from`,
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
        Long:  `Manage hypervisor hosts.

Commands:
  list        List all hosts
  get         Get a host by ID
  status      Get host status
  maintenance Toggle maintenance mode`,
    }

    hostCmd.AddCommand(hostListCmd, hostGetCmd)
    rootCmd.AddCommand(hostCmd)
}

func addStorageCommands() {
    storageCmd := &cobra.Command{
        Use:   "storage",
        Short: "Storage management commands",
        Long:  `Manage storage pools and disks.

Commands:
  pool          Storage pool commands
    list        List storage pools
    get         Get a storage pool
    create      Create a storage pool
    delete      Delete a storage pool
  disk          Disk commands
    list        List disks
    get         Get a disk
    resize      Resize a disk`,
    }

    diskCmd := &cobra.Command{
        Use:   "disk",
        Short: "Disk commands",
    }
    diskCmd.AddCommand(diskListCmd, diskGetCmd, diskResizeCmd)
    storageCmd.AddCommand(diskCmd)

    poolCmd := &cobra.Command{
        Use:   "pool",
        Short: "Storage pool commands",
    }
    poolCmd.AddCommand(storagePoolListCmd, storagePoolGetCmd, storagePoolCreateCmd, storagePoolDeleteCmd)

    storagePoolCreateCmd.Flags().StringP("type", "t", "dir", "Pool type: dir, lvm, zfs, nfs, iscsi, ceph")
    storagePoolCreateCmd.Flags().StringP("path", "p", "", "Pool path")

    storageCmd.AddCommand(poolCmd)
    rootCmd.AddCommand(storageCmd)
}

func addNetworkCommands() {
    networkCmd := &cobra.Command{
        Use:   "network",
        Short: "Network management commands",
        Long:  `Manage virtual networks.

Commands:
  list        List networks
  get         Get a network
  create      Create a network
  delete      Delete a network

Network create flags:
  --type    Network type: bridge, vlan, nat, ovs, macvlan
  --bridge  Bridge name (for bridge type)
  --vlan    VLAN ID (for vlan type)
  --subnet  Subnet (e.g. 192.168.1.0/24)`,
    }

    networkCmd.AddCommand(networkListCmd, networkGetCmd, networkCreateCmd, networkDeleteCmd)
    networkCreateCmd.Flags().StringP("type", "t", "bridge", "Network type")
    networkCreateCmd.Flags().StringP("bridge", "b", "", "Bridge name")
    networkCreateCmd.Flags().IntP("vlan", "", 0, "VLAN ID")
    networkCreateCmd.Flags().StringP("subnet", "s", "", "Subnet CIDR")

    rootCmd.AddCommand(networkCmd)
}

func addBackupCommands() {
    backupCmd := &cobra.Command{
        Use:   "backup",
        Short: "Backup management commands",
        Long:  `Manage VM backups.

Commands:
  list        List backups
  create      Create a backup
  restore     Restore a backup
  cancel      Cancel a backup

Backup create flags:
  --vm        VM ID (required)
  --name      Backup name
  --schedule  Schedule ID for scheduled backup`,
    }

    backupCmd.AddCommand(backupListCmd, backupCreateCmd, backupRestoreCmd, backupCancelCmd)
    backupCreateCmd.Flags().StringP("vm", "v", "", "VM ID (required)")
    backupCreateCmd.Flags().StringP("name", "n", "", "Backup name")
    backupCreateCmd.Flags().StringP("schedule", "s", "", "Schedule ID")

    rootCmd.AddCommand(backupCmd)
}

func addMigrationCommands() {
    migrateCmd := &cobra.Command{
        Use:   "migrate",
        Short: "VMware migration commands",
        Long:  `Import VMs from VMware environments.

Commands:
  import     Import from VMware (vcenter, ovf, vmx)
  list       List migration jobs

Import flags:
  --type         Source type: vcenter, ovf, vmx
  --file         File path (for ovf/vmx)
  --vcenter-url  vCenter API URL (for vcenter)
  --username     vCenter username (for vcenter)
  --password     vCenter password (for vcenter)`,
    }

    migrateCmd.AddCommand(migrateImportCmd, migrateListCmd)
    migrateImportCmd.Flags().StringP("type", "t", "vcenter", "Source type")
    migrateImportCmd.Flags().StringP("file", "f", "", "File path")
    migrateImportCmd.Flags().StringP("vcenter-url", "u", "", "vCenter API URL")
    migrateImportCmd.Flags().StringP("username", "U", "", "vCenter username")
    migrateImportCmd.Flags().StringP("password", "p", "", "vCenter password")

    rootCmd.AddCommand(migrateCmd)
}

func addSystemCommands() {
    rootCmd.AddCommand(healthCmd, versionCmd)
}

func init() {
    addAuthCommands()
    addVmCommands()
    addHostCommands()
    addStorageCommands()
    addNetworkCommands()
    addBackupCommands()
    addMigrationCommands()
    addSystemCommands()
}
