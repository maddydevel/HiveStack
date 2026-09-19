export interface User {
  id: string;
  tenant_id: string;
  username: string;
  role: string;
}

export interface TokenResponse {
  token: string;
  user: User;
  expires_at: string;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface Host {
  id: string;
  name: string;
  address: string;
  status: string;
  cpus: number;
  memory_bytes: number;
  vm_count: number;
}

export interface VM {
  id: string;
  name: string;
  host_id: string;
  status: string;
  role: string;
  cpus: number;
  memory_bytes: number;
  memory_reservation_bytes: number;
  ballooning_allowed: boolean;
  swap_allowed: boolean;
  hugepages_enabled: boolean;
  numa_policy: string;
}

export interface StoragePool {
  id: string;
  name: string;
  path: string;
  total_bytes: number;
  used_bytes: number;
}

export interface Network {
  id: string;
  name: string;
  bridge: string;
  subnet: string;
}

export interface Backup {
  id: string;
  vm_id: string;
  name: string;
  status: string;
  created_at: string;
}

export interface Violation {
  rule: string;
  field: string;
  expected: string;
  actual: string;
  severity: string;
  correctable: boolean;
}

export interface ComplianceResult {
  vm_id: string;
  check_type: string;
  passed: boolean;
  violations: Violation[];
  checked_at: string;
}

export interface Event {
  id: string;
  type: string;
  severity: string;
  message: string;
  created_at: string;
}

export interface CreateVMRequest {
  name: string;
  host_id: string;
  cpus: number;
  memory_bytes: number;
  memory_reservation_bytes: number;
  ballooning_allowed: boolean;
  swap_allowed: boolean;
  hugepages_enabled: boolean;
  numa_policy: string;
}

export interface CreateBackupRequest {
  vm_id: string;
  name: string;
}

export interface VMListResponse {
  data: VM[];
}

export interface HostListResponse {
  data: Host[];
}

export interface StoragePoolListResponse {
  data: StoragePool[];
}

export interface NetworkListResponse {
  data: Network[];
}

export interface BackupListResponse {
  data: Backup[];
}

export interface EventListResponse {
  data: Event[];
}
