import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Layout } from '../components/Layout';
import { StatusBadge } from '../components/StatusBadge';
import { getOrder, updateOrderStatus as apiUpdateStatus } from '../api/orders';
import type { Order, OrderStatus } from '../types';

export function OrderDetailsPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [order, setOrder] = useState<Order | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedStatus, setSelectedStatus] = useState<OrderStatus>('new');
  const [isUpdating, setIsUpdating] = useState(false);

  useEffect(() => {
    if (!id) return;

    setIsLoading(true);
    getOrder(parseInt(id))
      .then((data) => {
        setOrder(data);
        setSelectedStatus(data.status);
      })
      .catch(() => setError('Failed to load order'))
      .finally(() => setIsLoading(false));
  }, [id]);

  const handleUpdateStatus = async () => {
    if (!order || selectedStatus === order.status) return;

    setIsUpdating(true);
    try {
      const updated = await apiUpdateStatus(order.id, { status: selectedStatus });
      setOrder(updated);
    } catch (e) {
      setError('Failed to update order status');
    } finally {
      setIsUpdating(false);
    }
  };

  const formatDateTime = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleString();
  };

  if (isLoading) {
    return (
      <Layout>
        <div className="text-center py-12 text-slate-500">Loading order...</div>
      </Layout>
    );
  }

  if (error || !order) {
    return (
      <Layout>
        <div className="p-4 bg-red-50 border border-red-200 rounded-lg text-red-700">
          {error || 'Order not found'}
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="max-w-4xl mx-auto flex flex-col gap-6">
        <div className="flex justify-between items-center">
          <h2 className="text-3xl font-bold text-slate-800">Order #{order.dailyOrderNumber || order.id}</h2>
          <button
            onClick={() => navigate('/orders')}
            className="px-4 py-2 bg-slate-500 text-white rounded-lg font-medium hover:bg-slate-600 transition-all"
          >
            Back to Orders
          </button>
        </div>

        {/* Order Info Card */}
        <div className="bg-white rounded-xl shadow-md p-6">
          <h3 className="text-xl font-semibold text-slate-800 mb-4">Order Information</h3>
          <div className="divide-y divide-slate-200">
            <div className="py-3 flex gap-4 items-center">
              <strong className="min-w-32 text-slate-500">Vehicle:</strong>
              <span>{order.customerName || order.vehicle_description}</span>
            </div>
            <div className="py-3 flex gap-4 items-center">
              <strong className="min-w-32 text-slate-500">Status:</strong>
              <StatusBadge status={order.status} />
            </div>
            <div className="py-3 flex gap-4 items-center">
              <strong className="min-w-32 text-slate-500">Created:</strong>
              <span>{formatDateTime(order.created_at)}</span>
            </div>
            <div className="py-3 flex gap-4 items-center">
              <strong className="min-w-32 text-slate-500">Total:</strong>
              <span className="font-semibold text-blue-600">${order.total.toFixed(2)}</span>
            </div>
          </div>
        </div>

        {/* Order Items Card */}
        <div className="bg-white rounded-xl shadow-md p-6">
          <h3 className="text-xl font-semibold text-slate-800 mb-4">Order Items</h3>
          <ul className="divide-y divide-slate-200">
            {order.items.map((item, idx) => (
              <li key={idx} className="py-3 flex justify-between items-center">
                <span>
                  {item.quantity}x {item.menuItemName}
                </span>
                <span className="text-slate-600">${(item.price * item.quantity).toFixed(2)}</span>
              </li>
            ))}
          </ul>
        </div>

        {/* Update Status Card */}
        <div className="bg-white rounded-xl shadow-md p-6">
          <h3 className="text-xl font-semibold text-slate-800 mb-4">Update Status</h3>
          <div className="flex flex-col gap-4">
            <select
              value={selectedStatus}
              onChange={(e) => setSelectedStatus(e.target.value as OrderStatus)}
              className="p-3 border border-slate-300 rounded-md max-w-xs"
            >
              <option value="new">New</option>
              <option value="in-progress">In Progress</option>
              <option value="completed">Completed</option>
            </select>
            <button
              onClick={handleUpdateStatus}
              disabled={isUpdating || selectedStatus === order.status}
              className="self-start px-6 py-3 bg-blue-600 text-white rounded-lg font-semibold hover:bg-blue-700 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isUpdating ? 'Updating...' : 'Update Status'}
            </button>
          </div>
        </div>
      </div>
    </Layout>
  );
}
