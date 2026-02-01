import { useState } from 'react';
import { Layout } from '../components/Layout';
import { OrderForm } from '../components/OrderForm';
import { createOrder } from '../api/orders';
import type { CreateOrderRequest, Order } from '../types';

export function NewOrderPage() {
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [createdOrder, setCreatedOrder] = useState<Order | null>(null);

  const handleSubmit = async (data: CreateOrderRequest) => {
    setIsSubmitting(true);
    try {
      const order = await createOrder(data);
      setCreatedOrder(order);
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleCreateAnother = () => {
    setCreatedOrder(null);
  };

  if (createdOrder) {
    return (
      <Layout>
        <div className="max-w-2xl mx-auto">
          <div className="bg-gradient-to-r from-blue-600 to-blue-800 text-white p-8 rounded-xl text-center shadow-lg border-4 border-white">
            <div className="text-sm font-semibold uppercase tracking-wide mb-4 opacity-95">
              Your Order Number
            </div>
            <div className="text-7xl font-black leading-none my-2 drop-shadow-lg">
              #{createdOrder.dailyOrderNumber || createdOrder.id}
            </div>
            <div className="text-2xl font-bold mt-4 opacity-95">
              Total: ${createdOrder.total.toFixed(2)}
            </div>
            <button
              onClick={handleCreateAnother}
              className="mt-6 px-6 py-3 bg-slate-500 text-white rounded-lg font-semibold hover:bg-slate-600 transition-all"
            >
              Create Another Order
            </button>
          </div>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="bg-white rounded-xl shadow-md p-8 max-w-3xl mx-auto">
        <h2 className="text-3xl font-bold text-slate-800 mb-6">New Order</h2>
        <OrderForm onSubmit={handleSubmit} isSubmitting={isSubmitting} />
      </div>
    </Layout>
  );
}
