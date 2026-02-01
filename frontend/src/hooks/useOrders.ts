import { useState, useEffect, useCallback } from 'react';
import { getOrders, updateOrderStatus as apiUpdateStatus } from '../api/orders';
import { useOrdersWebSocket } from './useWebSocket';
import type { Order, OrderStatus, Stats, WebSocketMessage } from '../types';

export function useOrders() {
  const [orders, setOrders] = useState<Order[]>([]);
  const [stats, setStats] = useState<Stats | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Sort orders: in-progress first, then new, then completed
  const sortOrders = useCallback((orders: Order[]) => {
    const statusPriority: Record<OrderStatus, number> = {
      'in-progress': 1,
      'new': 2,
      'completed': 3,
    };

    return [...orders].sort((a, b) => {
      const aPriority = statusPriority[a.status] || 4;
      const bPriority = statusPriority[b.status] || 4;
      if (aPriority !== bPriority) {
        return aPriority - bPriority;
      }
      // Within same status, sort by ID ascending
      return a.id - b.id;
    });
  }, []);

  const loadOrders = useCallback(async () => {
    try {
      setIsLoading(true);
      const data = await getOrders();
      setOrders(sortOrders(data));
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load orders');
    } finally {
      setIsLoading(false);
    }
  }, [sortOrders]);

  const handleWebSocketMessage = useCallback((message: WebSocketMessage) => {
    if (message.type === 'order_new' && message.order) {
      setOrders((prev) => sortOrders([...prev, message.order!]));
    } else if (message.type === 'order_update' && message.order) {
      setOrders((prev) =>
        sortOrders(prev.map((o) => (o.id === message.order!.id ? message.order! : o)))
      );
    } else if (message.type === 'stats_update' && message.stats) {
      setStats(message.stats);
    }
  }, [sortOrders]);

  useOrdersWebSocket(handleWebSocketMessage);

  useEffect(() => {
    loadOrders();
  }, [loadOrders]);

  const updateOrderStatus = useCallback(async (id: number, status: OrderStatus) => {
    const updatedOrder = await apiUpdateStatus(id, { status });
    setOrders((prev) =>
      sortOrders(prev.map((o) => (o.id === id ? updatedOrder : o)))
    );
    return updatedOrder;
  }, [sortOrders]);

  return {
    orders,
    stats,
    isLoading,
    error,
    updateOrderStatus,
    reload: loadOrders,
  };
}
