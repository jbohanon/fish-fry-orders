import type { OrderStatus } from '../types';

interface StatusBadgeProps {
  status: OrderStatus;
}

const statusStyles: Record<OrderStatus, string> = {
  new: 'bg-red-100 text-red-800',
  'in-progress': 'bg-amber-100 text-amber-800',
  completed: 'bg-emerald-100 text-emerald-800',
};

export function StatusBadge({ status }: StatusBadgeProps) {
  return (
    <span
      className={`inline-block px-3 py-1.5 rounded-md text-xs font-semibold uppercase tracking-wide ${statusStyles[status]}`}
    >
      {status}
    </span>
  );
}
