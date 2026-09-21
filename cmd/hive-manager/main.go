// Copyright (c) 2026 Maddy AI Consultancy. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// Command hive-manager manages the HiveStack Manager service.
//
// The Manager is the central control plane for HiveStack — it manages
// tenants, enforces RBAC, orchestrates VM lifecycle, and serves the REST API.
//
// Usage:
//
//	hive-manager [command]
//
// Available Commands:
//
//	run         Start the HiveStack Manager service
//	init        Initialize a new HiveStack deployment (one-command bootstrap)
//	migrate     Run database migrations
//	validate    Validate configuration file
//	version     Print version and exit
package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"

	"github.com/maddydevel/HiveStack/internal/auth"
	"github.com/maddydevel/HiveStack/internal/config"
	"github.com/maddydevel/HiveStack/internal/db"
	hivetls "github.com/maddydevel/HiveStack/internal/tls"
	"github.com/spf13/cobra"
)

const version = "v0.1.0"

func main() {
	rootCmd := &cobra.Command{
		Use:   "hive-manager",
		Short: "HiveStack Manager — central control plane",
		Long: `HiveStack Manager is the central control plane for HiveStack.
It manages tenants, enforces RBAC, orchestrates VM lifecycle, and serves the REST API.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().String("config", "/etc/hivestack/manager.yaml", "path to manager config file")
	rootCmd.PersistentFlags().String("log-level", "info", "logging level (debug, info, warn, error)")

	rootCmd.AddCommand(
		newRunCmd(),
		newInitCmd(),
		newMigrateCmd(),
		newValidateCmd(),
		newVersionCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "hive-manager: %v\n", err)
		os.Exit(1)
	}
}

// newRunCmd returns the "run" subcommand, which starts the Manager service.
func newRunCmd() *cobra.Command {
	var cfgPath, logLevel string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start the HiveStack Manager service",
		Long:  `Start the HiveStack Manager service. This runs the API server, HA controller, and all subsystems.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runManager(cmd.Context(), cfgPath, logLevel)
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "/etc/hivestack/manager.yaml", "path to manager config file")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "logging level (debug, info, warn, error)")
	return cmd
}

// newInitCmd returns the "init" subcommand, which bootstraps a new deployment.
func newInitCmd() *cobra.Command {
	var (
		cfgPath    string
		tlsDir     string
		adminName  string
		adminEmail string
		adminPass  string
		tenantName string
		force      bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new HiveStack deployment (one-command bootstrap)",
		Long: `Initialize a new HiveStack deployment.

This command generates TLS certificates, creates default configuration files,
initializes the database schema, and creates an initial admin user.

Example:
    hive-manager init --tls-dir /etc/hivestack/tls --admin-email admin@example.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cfgPath, tlsDir, adminName, adminEmail, adminPass, tenantName, force)
		},
	}

	cmd.Flags().StringVar(&cfgPath, "config", "/etc/hivestack/manager.yaml", "path to write config file")
	cmd.Flags().StringVar(&tlsDir, "tls-dir", "/etc/hivestack/tls", "directory to store TLS certificates")
	cmd.Flags().StringVar(&adminName, "admin-name", "Administrator", "name for the admin user")
	cmd.Flags().StringVar(&adminEmail, "admin-email", "", "email for the admin user (required)")
	cmd.Flags().StringVar(&adminPass, "admin-password", "", "password for the admin user (if omitted, a random one is generated)")
	cmd.Flags().StringVar(&tenantName, "tenant-name", "Default", "name for the default tenant")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing config and certs")

	if cmd.MarkFlagRequired("admin-email") != nil {
		// MarkFlagRequired can fail if flag doesn't exist; ignore
	}

	return cmd
}

// newMigrateCmd returns the "migrate" subcommand.
func newMigrateCmd() *cobra.Command {
	var dsn string
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		Long: `Apply all pending SQL migrations to the database.

Migrations are tracked in a schema_migrations table so each migration runs only once.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrate(dsn)
		},
	}
	cmd.Flags().StringVar(&dsn, "dsn", "", "database connection string (overrides config)")
	return cmd
}

// newValidateCmd returns the "validate" subcommand.
func newValidateCmd() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		Long:  `Validate a HiveStack Manager configuration file for correctness.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(cfgPath)
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", "/etc/hivestack/manager.yaml", "path to config file")
	return cmd
}

// newVersionCmd returns the "version" subcommand.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and exit",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("HiveStack Manager %s (development build)\n", version)
		},
	}
}

// runInit initializes a new HiveStack deployment.
func runInit(cfgPath, tlsDir, adminName, adminEmail, adminPass, tenantName string, force bool) error {
	log.Println("HiveStack Manager init: starting")

	// Check if config already exists
	if _, err := os.Stat(cfgPath); err == nil && !force {
		return fmt.Errorf("config file %s already exists (use --force to overwrite)", cfgPath)
	}

	// Generate TLS certificates
	if err := os.MkdirAll(tlsDir, 0700); err != nil {
		return fmt.Errorf("create TLS directory: %w", err)
	}

	caCertPath := filepath.Join(tlsDir, "ca.crt")
	caKeyPath := filepath.Join(tlsDir, "ca.key")
	serverCertPath := filepath.Join(tlsDir, "server.crt")
	serverKeyPath := filepath.Join(tlsDir, "server.key")

	// Generate CA
	log.Println("Generating CA certificate...")
	if err := hivetls.GenerateCA(caCertPath, caKeyPath, "HiveStack Root CA"); err != nil {
		return fmt.Errorf("generate CA: %w", err)
	}

	// Generate server certificate
	log.Println("Generating server certificate...")
	caCert, caKey, err := hivetls.LoadCA(caCertPath, caKeyPath)
	if err != nil {
		return fmt.Errorf("load CA for signing: %w", err)
	}

	hostname, _ := os.Hostname()
	dnsNames := []string{"localhost", hostname}
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}

	if err := hivetls.GenerateServerCert(caCert, caKey, serverCertPath, serverKeyPath, "hivestack-manager", dnsNames, ips); err != nil {
		return fmt.Errorf("generate server cert: %w", err)
	}
	log.Printf("TLS certificates written to %s", tlsDir)

	// Generate admin password if not provided
	if adminPass == "" {
		adminPass, err = generateRandomPassword()
		if err != nil {
			return fmt.Errorf("generate admin password: %w", err)
		}
		log.Printf("Generated random admin password (save this — it will not be shown again)")
	}

	// Create default config
	cfg := config.DefaultServerConfig()
	cfg.TLSEnabled = true
	cfg.TLSCertFile = serverCertPath
	cfg.TLSKeyFile = serverKeyPath
	cfg.TLSCAFile = caCertPath

	// Write config file
	configDir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	configYAML := fmt.Sprintf(`# HiveStack Manager configuration
# Generated by hive-manager init

host: "%s"
port: %d

tls_enabled: true
tls_cert_file: "%s"
tls_key_file: "%s"
tls_ca_file: "%s"

database:
  dsn: "%s"

auth:
  jwt_secret: "%s"
  token_expiry: "%s"
`, cfg.Host, cfg.Port, cfg.TLSCertFile, cfg.TLSKeyFile, cfg.TLSCAFile,
		cfg.Database.DSN, cfg.Auth.JWTSecret, cfg.Auth.TokenExpiry)

	if err := os.WriteFile(cfgPath, []byte(configYAML), 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	log.Printf("Config file written to %s", cfgPath)

	// Connect to database and initialize schema
	dsn := os.Getenv("HIVESTACK_DSN")
	if dsn == "" {
		dsn = cfg.Database.DSN
	}

	log.Println("Connecting to database...")
	database, err := db.Open(dsn)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer database.Close()

	log.Println("Running database migrations...")
	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	// Create default tenant and admin user
	log.Println("Creating default tenant and admin user...")
	passwordHash, err := auth.HashPassword(adminPass)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	tenant := &db.Tenant{
		Name:   tenantName,
		Slug:   "default",
		Status: "active",
	}
	tenantID, err := database.CreateTenant(ctx, tenant)
	if err != nil {
		return fmt.Errorf("create tenant: %w", err)
	}

	user := &db.User{
		TenantID:     tenantID,
		Name:         adminName,
		Email:        adminEmail,
		PasswordHash: passwordHash,
		Role:         "admin",
	}
	userID, err := database.CreateUser(ctx, user)
	if err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}

	log.Println("")
	log.Println("=== HiveStack Manager initialized successfully ===")
	log.Printf("  Config:    %s", cfgPath)
	log.Printf("  TLS certs: %s", tlsDir)
	log.Printf("  Tenant:    %s (id=%s)", tenantName, tenantID)
	log.Printf("  Admin:     %s (id=%s)", adminEmail, userID)
	if adminPass != "" {
		log.Printf("  Password:  %s", adminPass)
	}
	log.Println("")
	log.Println("Start the manager with: hive-manager run")

	return nil
}

// runMigrate applies database migrations.
func runMigrate(dsn string) error {
	if dsn == "" {
		dsn = os.Getenv("HIVESTACK_DSN")
		if dsn == "" {
			dsn = "postgres://hivestack:***@localhost:5432/hivestack?sslmode=disable"
		}
	}

	log.Println("HiveStack Manager migrate: starting")
	log.Printf("Database DSN: %s", dsn)

	database, err := db.Open(dsn)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer database.Close()

	if err := database.Migrate(context.Background()); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	log.Println("HiveStack Manager migrate: completed successfully")
	return nil
}

// runValidate validates a configuration file.
func runValidate(cfgPath string) error {
	log.Printf("HiveStack Manager validate: checking %s", cfgPath)

	cfg, err := config.LoadServerConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	errs := cfg.Validate()
	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n%s", config.FormatErrors(errs))
	}

	log.Println("HiveStack Manager validate: OK")
	log.Printf("  Host: %s", cfg.Host)
	log.Printf("  Port: %d", cfg.Port)
	log.Printf("  TLS:  %v", cfg.TLSEnabled)
	log.Printf("  DSN:  %s", cfg.Database.DSN)
	return nil
}

// runManager starts the HiveStack Manager service (original behavior).
func runManager(ctx context.Context, cfgPath, logLevel string) error {
	log.Printf("HiveStack Manager starting (config: %s, log level: %s)", cfgPath, logLevel)

	dsn := os.Getenv("HIVESTACK_DSN")
	if dsn == "" {
		dsn = "postgres://hivestack:***@localhost:5432/hivestack?sslmode=disable"
		log.Printf("No HIVESTACK_DSN set — using default: %s", dsn)
	}

	database, err := db.Open(dsn)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer database.Close()

	log.Println("Database connected")

	log.Println("Database migrations: skipped (run manually or via migration tool)")

	// Original run logic continues...
	log.Println("HiveStack Manager run: full service startup not yet implemented")
	log.Println("  (Manager service startup will be wired into the 'run' subcommand)")

	<-ctx.Done()
	log.Println("HiveStack Manager stopped")
	return nil
}

// generateRandomPassword generates a cryptographically secure random password.
func generateRandomPassword() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	const length = 24
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}
