import { apiRequest } from './client';
import type {
  Session,
  SessionResponse,
  CreateSessionRequest,
  UpdateSessionRequest,
  SessionComparisonResponse,
  Order,
} from '../types';

export async function getCurrentSession(): Promise<SessionResponse> {
  return apiRequest<SessionResponse>('/api/session');
}

export async function createSession(data: CreateSessionRequest): Promise<Session> {
  return apiRequest<Session>('/api/session', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function getSession(id: number): Promise<Session> {
  return apiRequest<Session>(`/api/sessions/${id}`);
}

export async function updateSession(id: number, data: UpdateSessionRequest): Promise<Session> {
  return apiRequest<Session>(`/api/session/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

export async function closeSession(id: number): Promise<Session> {
  return apiRequest<Session>(`/api/session/${id}/close`, {
    method: 'POST',
  });
}

export async function getSessions(from?: string, to?: string): Promise<Session[]> {
  const params = new URLSearchParams();
  if (from) params.append('from', from);
  if (to) params.append('to', to);
  const query = params.toString();
  return apiRequest<Session[]>(`/api/sessions${query ? `?${query}` : ''}`);
}

export async function getSessionOrders(sessionId: number): Promise<Order[]> {
  return apiRequest<Order[]>(`/api/sessions/${sessionId}/orders`);
}

export async function compareSessions(sessionIds: number[]): Promise<SessionComparisonResponse> {
  const ids = sessionIds.join(',');
  return apiRequest<SessionComparisonResponse>(`/api/sessions/compare?ids=${ids}`);
}
