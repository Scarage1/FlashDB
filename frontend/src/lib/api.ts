const API_BASE = '/api';

export interface CommandResponse {
  result?: any;
  error?: string;
}

export interface ServerInfo {
  keys?: number;
  memory?: number;
  ops?: number;
  uptime?: number;
}

export interface KeysResponse {
  keys: string[];
}

export async function executeCommand(command: string): Promise<CommandResponse> {
  try {
    const response = await fetch(`${API_BASE}/command`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ command }),
    });
    return await response.json();
  } catch (error) {
    return { error: 'Failed to connect to server' };
  }
}

export async function getServerInfo(): Promise<ServerInfo> {
  try {
    const response = await fetch(`${API_BASE}/info`);
    return await response.json();
  } catch (error) {
    return {};
  }
}

export async function getKeys(): Promise<string[]> {
  try {
    const response = await fetch(`${API_BASE}/keys`);
    const data: KeysResponse = await response.json();
    return data.keys || [];
  } catch (error) {
    return [];
  }
}
