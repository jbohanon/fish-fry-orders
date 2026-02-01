const API_BASE = import.meta.env.VITE_API_BASE || '';

export interface ApiError {
  status: number;
  message: string;
}

export function createApiError(status: number, message: string): ApiError {
  return { status, message };
}

export function isApiError(error: unknown): error is ApiError {
  return typeof error === 'object' && error !== null && 'status' in error && 'message' in error;
}

export async function apiRequest<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const response = await fetch(`${API_BASE}${endpoint}`, {
    ...options,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  });

  if (!response.ok) {
    const message = await response.text().catch(() => `API Error: ${response.status}`);
    throw createApiError(response.status, message);
  }

  // Handle 204 No Content
  if (response.status === 204) {
    return undefined as T;
  }

  return response.json();
}
