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

export async function executeCommand(command: string): Promise<CommandResponse> {
  try {
    const response = await fetch(`${API_BASE}/execute`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ command }),
    });

    if (!response.ok) {
      return { error: `HTTP ${response.status}` };
    }

    const data: ExecuteAPIResponse = await response.json();
    if (!data.success) {
      return { error: data.error || 'Command failed' };
    }
    return { result: data.result };
  } catch (error) {
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
  } catch (error) {
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
  } catch (error) {
    return [];
  }
}
