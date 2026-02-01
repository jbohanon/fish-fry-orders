import { Link } from 'react-router-dom';
import { StatusBadge } from './StatusBadge';
import type { Order, OrderStatus } from '../types';

interface OrderTableProps {
  orders: Order[];
  onAdvanceStatus: (id: number, currentStatus: OrderStatus) => void;
}

const statusBorderColors: Record<OrderStatus, string> = {
  new: 'border-l-red-500',
  'in-progress': 'border-l-amber-500',
  completed: 'border-l-emerald-500',
};

export function OrderTable({ orders, onAdvanceStatus }: OrderTableProps) {
  if (orders.length === 0) {
    return (
      <div className="text-center py-12 text-slate-500 text-lg">
        No orders yet. Create a new order to get started!
      </div>
    );
  }

  const formatTime = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
  };

  const getNextStatus = (status: OrderStatus): OrderStatus | null => {
    if (status === 'new') return 'in-progress';
    if (status === 'in-progress') return 'completed';
    return null;
  };

  const getButtonText = (status: OrderStatus): string => {
    if (status === 'new') return 'Start Order';
    if (status === 'in-progress') return 'Complete Order';
    return '—';
  };

  return (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse">
        <thead className="bg-gradient-to-r from-blue-600 to-blue-800 text-white">
          <tr>
            <th className="p-4 text-left font-semibold text-sm uppercase tracking-wide">Order</th>
            <th className="p-4 text-left font-semibold text-sm uppercase tracking-wide">Vehicle</th>
            <th className="p-4 text-left font-semibold text-sm uppercase tracking-wide">Items</th>
            <th className="p-4 text-left font-semibold text-sm uppercase tracking-wide">Status</th>
            <th className="p-4 text-left font-semibold text-sm uppercase tracking-wide">Time</th>
            <th className="p-4 text-left font-semibold text-sm uppercase tracking-wide">Actions</th>
          </tr>
        </thead>
        <tbody>
          {orders.map((order) => (
            <tr
              key={order.id}
              className={`border-b border-slate-200 border-l-4 ${statusBorderColors[order.status]} hover:bg-slate-50 transition-colors`}
            >
              <td className="p-4">
                <Link
                  to={`/orders/${order.id}`}
                  className="text-blue-600 hover:text-blue-800 font-semibold hover:underline"
                >
                  #{order.dailyOrderNumber || order.id}
                </Link>
              </td>
              <td className="p-4">{order.customerName || order.vehicle_description}</td>
              <td className="p-4">
                <ul className="list-none p-0 m-0 text-sm text-slate-600">
                  {order.items.map((item, idx) => (
                    <li key={idx} className="py-0.5">
                      {item.quantity}x {item.menuItemName}
                    </li>
                  ))}
                </ul>
              </td>
              <td className="p-4">
                <StatusBadge status={order.status} />
              </td>
              <td className="p-4 text-slate-600">{formatTime(order.created_at)}</td>
              <td className="p-4">
                {getNextStatus(order.status) ? (
                  <button
                    onClick={() => onAdvanceStatus(order.id, order.status)}
                    className="px-4 py-2 text-sm rounded-lg bg-blue-600 text-white font-medium hover:bg-blue-700 hover:-translate-y-0.5 transition-all shadow-sm"
                  >
                    {getButtonText(order.status)}
                  </button>
                ) : (
                  <span className="text-slate-400">—</span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
