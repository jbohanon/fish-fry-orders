import { useState, useEffect, useCallback } from 'react';
import { getOrders, updateOrderStatus as apiUpdateStatus } from '../api/orders';
import { getCurrentSession } from '../api/sessions';
import { useOrdersWebSocket } from './useWebSocket';
import type { Order, OrderStatus, Stats, Session, WebSocketMessage } from '../types';

export function useOrders() {
  const [orders, setOrders] = useState<Order[]>([]);
  const [stats, setStats] = useState<Stats | null>(null);
  const [session, setSession] = useState<Session | null>(null);
  const [hasActiveSession, setHasActiveSession] = useState<boolean>(false);
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
      // Within same status, sort by daily order number ascending
      return (a.dailyOrderNumber || a.id) - (b.dailyOrderNumber || b.id);
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

  const loadSession = useCallback(async () => {
    try {
      const response = await getCurrentSession();
      setHasActiveSession(response.active);
      setSession(response.session || null);
    } catch (e) {
      // Session fetch failure is not critical
      console.error('Failed to load session:', e);
    }
  }, []);

  const handleWebSocketMessage = useCallback((message: WebSocketMessage) => {
    if (message.type === 'order_new' && message.order) {
      setOrders((prev) => sortOrders([...prev, message.order!]));
    } else if (message.type === 'order_update' && message.order) {
      setOrders((prev) =>
        sortOrders(prev.map((o) => (o.id === message.order!.id ? message.order! : o)))
      );
    } else if (message.type === 'stats_update' && message.stats) {
      setStats(message.stats);
    } else if (message.type === 'session_update') {
      setHasActiveSession(message.active ?? true);
      if (message.session) {
        setSession(message.session);
      }
    } else if (message.type === 'session_closed') {
      setHasActiveSession(false);
      if (message.session) {
        setSession(message.session);
      }
      // Clear orders when session closes (they belong to the old session)
      setOrders([]);
    }
  }, [sortOrders]);

  useOrdersWebSocket(handleWebSocketMessage);

  useEffect(() => {
    loadOrders();
    loadSession();
  }, [loadOrders, loadSession]);

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
    session,
    hasActiveSession,
    isLoading,
    error,
    updateOrderStatus,
    reload: loadOrders,
    reloadSession: loadSession,
  };
}
