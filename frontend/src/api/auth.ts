import { apiRequest } from './client';
import type { User } from '../types';

export async function login(password: string): Promise<User> {
  return apiRequest<User>('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ password }),
  });
}

export async function logout(): Promise<void> {
  return apiRequest<void>('/api/auth/logout', {
    method: 'POST',
  });
}

export async function checkAuth(): Promise<User> {
  return apiRequest<User>('/api/auth/check');
}
