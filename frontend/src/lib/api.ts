const API_BASE = '/api/v1';

export interface CommandResponse {
  result?: unknown;
  error?: string;
}

export interface ServerInfo {
  keys: number;
  memory: number;
  ops: number;
  uptime: number;
}

interface ExecuteAPIResponse {
  success: boolean;
  result?: unknown;
  error?: string;
}

interface StatsAPIResponse {
  keys?: number;
  memory_used?: number;
  total_commands?: number;
  uptime?: number;
}

interface KeyInfo {
  key?: string;
}

interface KeysAPIResponse {
  keys?: Array<string | KeyInfo>;
}

function parseCommandInput(input: string): string[] {
  const parts: string[] = [];
  let current = '';
  let inQuote = false;
  let quoteChar = '';

  for (let i = 0; i < input.length; i++) {
    const char = input[i];

    if (inQuote) {
      if (char === quoteChar) {
        inQuote = false;
      } else {
        current += char;
      }
      continue;
    }

    if (char === '"' || char === "'") {
      inQuote = true;
      quoteChar = char;
      continue;
    }

    if (char === ' ') {
      if (current.length > 0) {
        parts.push(current);
        current = '';
      }
      continue;
    }

    current += char;
  }

  if (current.length > 0) {
    parts.push(current);
  }

  return parts;
}

export async function executeCommand(command: string): Promise<CommandResponse> {
  try {
    const parsed = parseCommandInput(command.trim());
    const payload =
      parsed.length > 0
        ? { command: parsed[0], args: parsed.slice(1) }
        : { command };

    const response = await fetch(`${API_BASE}/execute`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });

    if (!response.ok) {
      return { error: `HTTP ${response.status}` };
    }

    const data: ExecuteAPIResponse = await response.json();
    if (!data.success) {
      return { error: data.error || 'Command failed' };
    }
    return { result: data.result };
  } catch {
    return { error: 'Failed to connect to server' };
  }
}

export async function getServerInfo(): Promise<ServerInfo> {
  try {
    const response = await fetch(`${API_BASE}/stats`);
    if (!response.ok) {
      return { keys: 0, memory: 0, ops: 0, uptime: 0 };
    }

    const data: StatsAPIResponse = await response.json();
    return {
      keys: data.keys || 0,
      memory: data.memory_used || 0,
      ops: data.total_commands || 0,
      uptime: data.uptime || 0,
    };
  } catch {
    return { keys: 0, memory: 0, ops: 0, uptime: 0 };
  }
}

export async function getKeys(): Promise<string[]> {
  try {
    const response = await fetch(`${API_BASE}/keys`);
    if (!response.ok) {
      return [];
    }

    const data: KeysAPIResponse = await response.json();
    const keys = data.keys || [];
    return keys
      .map((entry) => (typeof entry === 'string' ? entry : entry.key || ''))
      .filter((key) => key.length > 0);
  } catch {
    return [];
  }
}

/* ─── Phase 6: Hot Keys ─── */

export interface HotKeyEntry {
  key: string;
  count: number;
}

export async function getHotKeys(n = 20): Promise<HotKeyEntry[]> {
  try {
    const response = await fetch(`${API_BASE}/hotkeys?n=${n}`);
    if (!response.ok) return [];
    const data = await response.json();
    return data.hotkeys || [];
  } catch {
    return [];
  }
}

/* ─── Phase 6: Time Series ─── */

export interface TSDataPoint {
  ts: number;
  val: number;
}

export interface TSInfo {
  total_samples: number;
  first_timestamp: number;
  last_timestamp: number;
  retention_ms: number;
  memory_bytes: number;
}

export async function tsAdd(key: string, value: number, timestamp?: number): Promise<number | null> {
  try {
    const body: Record<string, unknown> = { value };
    if (timestamp) body.timestamp = timestamp;
    const response = await fetch(`${API_BASE}/timeseries/${encodeURIComponent(key)}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!response.ok) return null;
    const data = await response.json();
    return data.timestamp || null;
  } catch {
    return null;
  }
}

export async function tsGet(key: string): Promise<TSDataPoint | null> {
  try {
    const response = await fetch(`${API_BASE}/timeseries/${encodeURIComponent(key)}`);
    if (!response.ok) return null;
    return await response.json();
  } catch {
    return null;
  }
}

export async function tsRange(key: string, start: number, end: number): Promise<TSDataPoint[]> {
  try {
    const response = await fetch(
      `${API_BASE}/timeseries/${encodeURIComponent(key)}?from=${start}&to=${end}`
    );
    if (!response.ok) return [];
    const data = await response.json();
    return data.points || [];
  } catch {
    return [];
  }
}

export async function tsInfo(key: string): Promise<TSInfo | null> {
  try {
    const response = await fetch(
      `${API_BASE}/timeseries/${encodeURIComponent(key)}?info=true`
    );
    if (!response.ok) return null;
    return await response.json();
  } catch {
    return null;
  }
}

export async function tsDelete(key: string): Promise<boolean> {
  try {
    const response = await fetch(`${API_BASE}/timeseries/${encodeURIComponent(key)}`, {
      method: 'DELETE',
    });
    return response.ok;
  } catch {
    return false;
  }
}

/* ─── Phase 6: CDC ─── */

export interface CDCEvent {
  id: number;
  op: string;
  key: string;
  value: string;
  ts: number;
}

export interface CDCStats {
  total_events: number;
  buffer_size: number;
  buffer_cap: number;
  subscribers: number;
}

export interface CDCResponse {
  events: CDCEvent[];
  stats: CDCStats;
}

export async function getCDCEvents(after?: number, n = 50): Promise<CDCResponse> {
  try {
    const params = new URLSearchParams();
    if (after !== undefined) params.set('after', String(after));
    else params.set('n', String(n));
    const response = await fetch(`${API_BASE}/cdc?${params}`);
    if (!response.ok) return { events: [], stats: { total_events: 0, buffer_size: 0, buffer_cap: 0, subscribers: 0 } };
    return await response.json();
  } catch {
    return { events: [], stats: { total_events: 0, buffer_size: 0, buffer_cap: 0, subscribers: 0 } };
  }
}

export function subscribeCDC(onEvent: (event: CDCEvent) => void): () => void {
  const eventSource = new EventSource(`${API_BASE}/cdc/stream`);
  eventSource.onmessage = (e) => {
    try {
      const event: CDCEvent = JSON.parse(e.data);
      onEvent(event);
    } catch { /* ignore parse errors */ }
  };
  return () => eventSource.close();
}

/* ─── Phase 6: Snapshots ─── */

export interface SnapshotMeta {
  id: string;
  created_at: string;
  size_bytes: number;
  file_path: string;
}

export async function listSnapshots(): Promise<SnapshotMeta[]> {
  try {
    const response = await fetch(`${API_BASE}/snapshots`);
    if (!response.ok) return [];
    const data = await response.json();
    return data.snapshots || [];
  } catch {
    return [];
  }
}

export async function createSnapshot(name?: string): Promise<SnapshotMeta | null> {
  try {
    const body: Record<string, string> = {};
    if (name) body.id = name;
    const response = await fetch(`${API_BASE}/snapshots`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!response.ok) return null;
    return await response.json();
  } catch {
    return null;
  }
}

export async function restoreSnapshot(id: string): Promise<boolean> {
  try {
    const response = await fetch(`${API_BASE}/snapshots`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id }),
    });
    return response.ok;
  } catch {
    return false;
  }
}

export async function deleteSnapshot(id: string): Promise<boolean> {
  try {
    const response = await fetch(`${API_BASE}/snapshots?id=${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
    return response.ok;
  } catch {
    return false;
  }
}

/* ─── Phase 6: Benchmark ─── */

export interface BenchmarkResult {
  operations: number;
  duration_ns: number;
  ops_per_sec: number;
  avg_latency_ns: number;
  // Per-operation breakdown
  set_ops_per_sec: number;
  get_ops_per_sec: number;
  del_ops_per_sec: number;
  // Latency percentiles
  p50_latency_ns: number;
  p99_latency_ns: number;
  p999_latency_ns: number;
  // Concurrency
  concurrency: number;
  concurrent_ops_per_sec?: number;
  concurrent_avg_latency_ns?: number;
  scale_factor?: number;
}

export async function runBenchmark(operations = 1000): Promise<BenchmarkResult | null> {
  try {
    const response = await fetch(`${API_BASE}/benchmark`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ operations }),
    });
    if (!response.ok) return null;
    return await response.json();
  } catch {
    return null;
  }
}
