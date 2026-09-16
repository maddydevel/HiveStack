// Copyright (c) 2026 Maddy AI Consultancy. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// Command hive-manager starts the HiveStack Manager service.
//
// The Manager is the central control plane for HiveStack — it manages
// tenants, enforces RBAC, orchestrates VM lifecycle, and serves the REST API.
//
// Usage:
//
//     hive-manager [flags]
//
// Flags:
//
//     --config path     Path to manager config file (default: /etc/hivestack/manager.yaml)
//     --log-level level Set logging level (debug, info, warn, error)
//     --version         Print version and exit
package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/maddydevel/HiveStack/internal/db"
    "github.com/maddydevel/HiveStack/internal/manager"
)

func main() {
    cfgPath := flag.String("config", "/etc/hivestack/manager.yaml", "path to manager config file")
    logLevel := flag.String("log-level", "info", "logging level (debug, info, warn, error)")
    showVersion := flag.Bool("version", false, "print version and exit")
    flag.Parse()

    if *showVersion {
        fmt.Printf("HiveStack Manager v0.1.0 (development build)\n")
        os.Exit(0)
    }

    log.Printf("HiveStack Manager starting (config: %s, log level: %s)", *cfgPath, *logLevel)

    // Initialize database connection
    // In production, the DSN comes from config file
    dsn := os.Getenv("HIVESTACK_DSN")
    if dsn == "" {
        dsn = "postgres://hivestack:hivestack@localhost:5432/hivestack?sslmode=disable"
        log.Printf("No HIVESTACK_DSN set — using default: %s", dsn)
    }

    db, err := db.Open(dsn)
    if err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }
    defer db.Close()

    log.Println("Database connected")

    // Run migrations
    // In production: migrate from embedded SQL files
    log.Println("Database migrations: skipped (run manually or via migration tool)")

    // Create and start the manager
    mgr, err := manager.New(db)
    if err != nil {
        log.Fatalf("Failed to create manager: %v", err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        sig := <-sigChan
        log.Printf("Received signal %v, shutting down...", sig)
        cancel()
    }()

    if err := mgr.Run(ctx, "0.0.0.0:8080"); err != nil && err != context.Canceled {
        log.Fatalf("Manager error: %v", err)
    }

    log.Println("HiveStack Manager stopped")
}
