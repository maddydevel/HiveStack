package config

// NodeConfig holds the HiveStack Node Agent configuration.
type NodeConfig struct {
    // ManagerAddress is the gRPC address of the Manager.
    ManagerAddress string `mapstructure:"manager_address"`
    // NodeID is this node's identifier.
    NodeID string `mapstructure:"node_id"`
    // HeartbeatInterval is the status report interval.
    HeartbeatInterval string `mapstructure:"heartbeat_interval"`
    // TLSConfig holds TLS configuration.
    TLSConfig TLSConfig `mapstructure:"tls"`
    // LibvirtURI is the libvirt connection URI.
    LibvirtURI string `mapstructure:"libvirt_uri"`
}

// TLSConfig holds TLS settings.
type TLSConfig struct {
    CertFile string `mapstructure:"cert_file"`
    KeyFile  string `mapstructure:"key_file"`
    CAFile   string `mapstructure:"ca_file"`
}
