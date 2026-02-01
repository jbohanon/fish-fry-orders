import { Layout } from '../components/Layout';
import { OrderTable } from '../components/OrderTable';
import { useOrders } from '../hooks/useOrders';
import type { OrderStatus } from '../types';

export function OrdersPage() {
  const { orders, isLoading, error, updateOrderStatus } = useOrders();

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
        <h2 className="text-3xl font-bold text-slate-800 mb-6">Orders</h2>
        {isLoading ? (
          <div className="text-center py-12 text-slate-500">Loading orders...</div>
        ) : error ? (
          <div className="p-4 bg-red-50 border border-red-200 rounded-lg text-red-700">{error}</div>
        ) : (
          <div className="bg-white rounded-xl shadow-md overflow-hidden">
            <OrderTable orders={orders} onAdvanceStatus={handleAdvanceStatus} />
          </div>
        )}
      </div>
    </Layout>
  );
}
