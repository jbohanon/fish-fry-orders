import { apiRequest } from './client';
import type { MenuItem } from '../types';

export async function getMenuItems(): Promise<MenuItem[]> {
  return apiRequest<MenuItem[]>('/api/menu-items');
}

export async function getMenuItem(id: string): Promise<MenuItem> {
  return apiRequest<MenuItem>(`/api/menu-items/${id}`);
}

export async function createMenuItem(data: Omit<MenuItem, 'id'>): Promise<MenuItem> {
  return apiRequest<MenuItem>('/api/menu-items', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function updateMenuItem(id: string, data: Partial<MenuItem>): Promise<MenuItem> {
  return apiRequest<MenuItem>(`/api/menu-items/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

export async function deleteMenuItem(id: string): Promise<void> {
  return apiRequest<void>(`/api/menu-items/${id}`, {
    method: 'DELETE',
  });
}

export async function updateMenuItemsOrder(itemOrders: Record<string, number>): Promise<void> {
  return apiRequest<void>('/api/menu-items/order', {
    method: 'PUT',
    body: JSON.stringify({ itemOrders }),
  });
}
