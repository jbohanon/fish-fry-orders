import { apiRequest } from './client';
import type { Order, CreateOrderRequest, UpdateStatusRequest, PurgeOrdersRequest, PurgeOrdersResponse } from '../types';

export async function getOrders(): Promise<Order[]> {
  return apiRequest<Order[]>('/api/orders');
}

export async function getOrder(id: number): Promise<Order> {
  return apiRequest<Order>(`/api/orders/${id}`);
}

export async function createOrder(data: CreateOrderRequest): Promise<Order> {
  return apiRequest<Order>('/api/orders', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function updateOrderStatus(id: number, data: UpdateStatusRequest): Promise<Order> {
  return apiRequest<Order>(`/api/orders/${id}/status`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

export async function purgeOrders(data: PurgeOrdersRequest): Promise<PurgeOrdersResponse> {
  return apiRequest<PurgeOrdersResponse>('/api/orders/purge', {
    method: 'DELETE',
    body: JSON.stringify(data),
  });
}
