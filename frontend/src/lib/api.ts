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
