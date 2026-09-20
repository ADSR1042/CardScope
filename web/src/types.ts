export type Metric = number | null;
export type Process = {
  pid: number;
  uid: string;
  user: string;
  name: string;
  created: number;
  memory: Metric;
  first_seen: number;
  last_seen: number;
};
export type GPU = {
  uuid: string;
  name: string;
  index: number;
  pci: string;
  util: Metric;
  memory_used: Metric;
  memory_total: Metric;
  temperature: Metric;
  power: Metric;
  ecc: Metric;
  status: string;
  fields: Record<string, string>;
  processes: Process[] | null;
  process_status: string;
  source: string;
  seen_at: number;
};
export type Net = {
  name: string;
  rx: number;
  tx: number;
  rx_rate: Metric;
  tx_rate: Metric;
  up: boolean;
  primary: boolean;
};
export type Snapshot = {
  at: number;
  hostname: string;
  cache_dropped: number;
  gpu_status: string;
  gpus: GPU[] | null;
  system: {
    cpu: Metric;
    cores: number;
    load: Metric;
    memory_total: number;
    memory_available: number;
    swap_total: number;
    swap_used: number;
    wsl: boolean;
    networks: Net[] | null;
    disks: { path: string; device: string; total: number; free: number; status: string }[] | null;
    io: { device: string; read_rate: Metric; write_rate: Metric }[] | null;
    errors: Record<string, string> | null;
  };
};
export type Node = {
  id: string;
  name: string;
  expected: number;
  networks: string[] | null;
  tags: string[];
  snapshot: Snapshot;
  sample_at: number;
  received_at: number;
  online: boolean;
  enrolled: boolean;
  created: number;
};
export type User = { username: string; role: 'admin' | 'viewer'; csrf: string };
export type Stat = {
  node: string;
  uuid: string;
  model: string;
  user_id: string;
  user: string;
  occupied_hours: number;
  weighted_hours: number;
  occupancy_seconds: number;
  util_seconds: number;
  expected_seconds: number;
  coverage: number;
  util_coverage: number;
};

export type LoginCopy = { eyebrow: string; title: string; description: string; caption: string };
