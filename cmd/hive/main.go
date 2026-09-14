// HiveStack CLI — hive
package main

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "hive",
    Short: "HiveStack management CLI",
    Long:  "CLI for managing HiveStack virtualization infrastructure — VMs, hosts, storage, networks, backups.",
    RunE: func(cmd *cobra.Command, args []string) error {
        if len(args) == 0 {
            return fmt.Errorf("subcommand required; see 'hive --help'")
        }
        return fmt.Errorf("unknown subcommand: %s", args[0])
    },
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}
