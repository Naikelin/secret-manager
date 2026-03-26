// API client utility for making authenticated requests

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export interface ApiError {
  error: string;
}

// Get auth token from localStorage
function getAuthToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem('auth_token');
}

// Make authenticated API request
export async function apiRequest<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const token = getAuthToken();
  
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };

  // Merge existing headers
  if (options.headers) {
    const existingHeaders = new Headers(options.headers);
    existingHeaders.forEach((value, key) => {
      headers[key] = value;
    });
  }

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const url = `${API_URL}${endpoint}`;
  
  // Log request details for debugging (only in development)
  if (process.env.NODE_ENV === 'development') {
    console.log('[API Request]', options.method || 'GET', url);
  }

  try {
    const response = await fetch(url, {
      ...options,
      headers,
    });

    if (!response.ok) {
      const error: ApiError = await response.json().catch(() => ({
        error: `HTTP ${response.status}: ${response.statusText}`,
      }));
      
      // Log error details for debugging
      console.error('[API Error]', {
        endpoint,
        status: response.status,
        statusText: response.statusText,
        error: error.error,
        url,
      });
      
      throw new Error(error.error || `HTTP ${response.status}: ${response.statusText}`);
    }

    return response.json();
  } catch (err) {
    // Log network errors
    console.error('[API Network Error]', {
      endpoint,
      url,
      error: err instanceof Error ? err.message : 'Unknown error',
    });
    
    // Re-throw with better error message
    if (err instanceof Error) {
      // If it's already an Error with a good message, keep it
      if (err.message.includes('HTTP') || err.message.includes('Failed to fetch')) {
        throw err;
      }
      throw new Error(`Network error: ${err.message}`);
    }
    
    throw new Error('Network error: Failed to connect to API');
  }
}

// Logout helper
export function logout() {
  if (typeof window === 'undefined') return;
  localStorage.removeItem('auth_token');
  localStorage.removeItem('user');
  window.location.href = '/auth/login';
}

// Check if user is authenticated
export function isAuthenticated(): boolean {
  return getAuthToken() !== null;
}

// Get current user from localStorage
export function getCurrentUser() {
  if (typeof window === 'undefined') return null;
  const userStr = localStorage.getItem('user');
  return userStr ? JSON.parse(userStr) : null;
}

// ===== Data Types =====

export interface Namespace {
  id: string;
  name: string;
  cluster: string;
  environment: string;
  created_at: string;
}

export interface Secret {
  id: string;
  secret_name: string;
  namespace_id: string;
  data: Record<string, string>;
  status: 'draft' | 'published' | 'drifted';
  edited_by: string;
  edited_at: string;
  published_by?: string;
  published_at?: string;
  commit_sha?: string;
  created_at: string;
  updated_at: string;
}

export interface DriftEvent {
  id: string;
  secret_name: string;
  detected_at: string;
  git_version: Record<string, any>;
  k8s_version: Record<string, any>;
  diff: {
    differences?: string[];
    message?: string;
    keys_changed?: string[];
    [key: string]: any;
  };
  resolved_at?: string;
  resolved_by?: string;
  resolution_action?: string;
}

export interface DriftEventsResponse {
  namespace: string;
  total: number;
  events: DriftEvent[];
}

export interface AuditLog {
  id: string;
  timestamp: string;
  user_id?: string;
  action_type: string;
  resource_type: string;
  resource_name: string;
  namespace_id?: string;
  metadata?: any;
  user?: {
    id: string;
    email: string;
    name: string;
  };
  namespace?: {
    id: string;
    name: string;
  };
}

export interface AuditLogsResponse {
  total: number;
  page: number;
  limit: number;
  logs: AuditLog[];
}

export interface AuditLogFilters {
  user_id?: string;
  action?: string;
  resource_type?: string;
  resource_name?: string;
  namespace_id?: string;
  start_date?: string;
  end_date?: string;
  page?: number;
  limit?: number;
}

export interface SyncStatus {
  namespace: string;
  git_commit: string;
  flux_commit: string;
  synced: boolean;
  last_sync: string;
  error: string | null;
}

// ===== API Methods =====

export const api = {
  // Namespaces
  async getNamespaces(): Promise<Namespace[]> {
    return apiRequest<Namespace[]>('/api/v1/namespaces');
  },

  // Secrets
  async getSecrets(namespaceId: string): Promise<Secret[]> {
    return apiRequest<Secret[]>(`/api/v1/namespaces/${namespaceId}/secrets`);
  },

  async getSecret(namespaceId: string, name: string): Promise<Secret> {
    return apiRequest<Secret>(`/api/v1/namespaces/${namespaceId}/secrets/${name}`);
  },

  async createSecret(
    namespaceId: string,
    data: { name: string; data: Record<string, string> }
  ): Promise<Secret> {
    return apiRequest<Secret>(`/api/v1/namespaces/${namespaceId}/secrets`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  async updateSecret(
    namespaceId: string,
    name: string,
    data: Record<string, string>
  ): Promise<Secret> {
    return apiRequest<Secret>(`/api/v1/namespaces/${namespaceId}/secrets/${name}`, {
      method: 'PUT',
      body: JSON.stringify({ data }),
    });
  },

  async deleteSecret(namespaceId: string, name: string): Promise<void> {
    return apiRequest<void>(`/api/v1/namespaces/${namespaceId}/secrets/${name}`, {
      method: 'DELETE',
    });
  },

  async publishSecret(namespaceId: string, name: string): Promise<Secret> {
    return apiRequest<Secret>(`/api/v1/namespaces/${namespaceId}/secrets/${name}/publish`, {
      method: 'POST',
    });
  },

  // Drift Detection
  async triggerDriftCheck(namespaceId: string): Promise<void> {
    return apiRequest<void>(`/api/v1/namespaces/${namespaceId}/drift-check`, {
      method: 'POST',
    });
  },

  async getDriftEvents(namespaceId: string): Promise<DriftEventsResponse> {
    return apiRequest<DriftEventsResponse>(`/api/v1/namespaces/${namespaceId}/drift-events`);
  },

  async getDriftComparison(driftEventId: string): Promise<{ git_data: Record<string, string>; k8s_data: Record<string, string> }> {
    return apiRequest<{ git_data: Record<string, string>; k8s_data: Record<string, string> }>(
      `/api/v1/drift-events/${driftEventId}/compare`
    );
  },

  // Drift Resolution (admin only)
  async syncFromGit(driftEventId: string): Promise<void> {
    return apiRequest<void>(`/api/v1/drift-events/${driftEventId}/sync-from-git`, {
      method: 'POST',
    });
  },

  async importToGit(driftEventId: string): Promise<void> {
    return apiRequest<void>(`/api/v1/drift-events/${driftEventId}/import-to-git`, {
      method: 'POST',
    });
  },

  async markDriftResolved(driftEventId: string): Promise<void> {
    return apiRequest<void>(`/api/v1/drift-events/${driftEventId}/mark-resolved`, {
      method: 'POST',
    });
  },

  // Audit Logs
  async getAuditLogs(filters?: AuditLogFilters): Promise<AuditLogsResponse> {
    const params = new URLSearchParams();
    
    if (filters) {
      if (filters.user_id) params.append('user_id', filters.user_id);
      if (filters.action) params.append('action', filters.action);
      if (filters.resource_type) params.append('resource_type', filters.resource_type);
      if (filters.resource_name) params.append('resource_name', filters.resource_name);
      if (filters.namespace_id) params.append('namespace_id', filters.namespace_id);
      if (filters.start_date) params.append('start_date', filters.start_date);
      if (filters.end_date) params.append('end_date', filters.end_date);
      if (filters.page) params.append('page', filters.page.toString());
      if (filters.limit) params.append('limit', filters.limit.toString());
    }

    const query = params.toString();
    const endpoint = query ? `/api/v1/audit-logs?${query}` : '/api/v1/audit-logs';
    
    return apiRequest<AuditLogsResponse>(endpoint);
  },

  async exportAuditLogsCSV(filters?: AuditLogFilters): Promise<void> {
    const params = new URLSearchParams();
    
    if (filters) {
      if (filters.user_id) params.append('user_id', filters.user_id);
      if (filters.action) params.append('action', filters.action);
      if (filters.resource_type) params.append('resource_type', filters.resource_type);
      if (filters.resource_name) params.append('resource_name', filters.resource_name);
      if (filters.namespace_id) params.append('namespace_id', filters.namespace_id);
      if (filters.start_date) params.append('start_date', filters.start_date);
      if (filters.end_date) params.append('end_date', filters.end_date);
    }

    const query = params.toString();
    const endpoint = query ? `/api/v1/audit-logs/export?${query}` : '/api/v1/audit-logs/export';
    
    const token = getAuthToken();
    const headers: Record<string, string> = {};
    
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    
    const response = await fetch(`${API_URL}${endpoint}`, { headers });
    
    if (!response.ok) {
      throw new Error('Failed to export audit logs');
    }
    
    // Get the blob and trigger download
    const blob = await response.blob();
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `audit-logs-${new Date().toISOString().split('T')[0]}.csv`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    window.URL.revokeObjectURL(url);
  },

  // FluxCD Sync Status
  async getSyncStatus(namespaceId: string): Promise<SyncStatus> {
    return apiRequest<SyncStatus>(`/api/v1/namespaces/${namespaceId}/sync-status`);
  },
};
