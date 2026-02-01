import { useState } from 'react';
import { Layout } from '../components/Layout';
import { OrderTable } from '../components/OrderTable';
import { useOrders } from '../hooks/useOrders';
import type { OrderStatus } from '../types';

export function OrdersPage() {
  const { orders, isLoading, error, updateOrderStatus } = useOrders();
  const [hideCompleted, setHideCompleted] = useState(false);

  const filteredOrders = hideCompleted
    ? orders.filter((order) => order.status !== 'completed')
    : orders;

  const handleAdvanceStatus = async (id: number, currentStatus: OrderStatus) => {
    let nextStatus: OrderStatus;
    if (currentStatus === 'new') {
      nextStatus = 'in-progress';
    } else if (currentStatus === 'in-progress') {
      nextStatus = 'completed';
    } else {
      return;
    }

    try {
      await updateOrderStatus(id, nextStatus);
    } catch (e) {
      console.error('Failed to update order status:', e);
    }
  };

  return (
    <Layout>
      <div className="orders-container">
        <div className="flex justify-between items-center mb-6">
          <h2 className="text-3xl font-bold text-slate-800">Orders</h2>
          <label className="flex items-center gap-2 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={hideCompleted}
              onChange={(e) => setHideCompleted(e.target.checked)}
              className="w-4 h-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500"
            />
            <span className="text-sm text-slate-600">Hide completed</span>
          </label>
        </div>
        {isLoading ? (
          <div className="text-center py-12 text-slate-500">Loading orders...</div>
        ) : error ? (
          <div className="p-4 bg-red-50 border border-red-200 rounded-lg text-red-700">{error}</div>
        ) : (
          <div className="bg-white rounded-xl shadow-md overflow-hidden">
            <OrderTable orders={filteredOrders} onAdvanceStatus={handleAdvanceStatus} />
          </div>
        )}
      </div>
    </Layout>
  );
}
