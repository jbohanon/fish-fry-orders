import { useState, useEffect } from 'react';
import { getMenuItems } from '../api/menu';
import type { MenuItem, CreateOrderRequest } from '../types';

interface OrderFormProps {
  onSubmit: (data: CreateOrderRequest) => Promise<void>;
  isSubmitting: boolean;
}

interface OrderItemInput {
  menuItemId: string;
  quantity: number | string; // Allow string for intermediate empty state during editing
}

export function OrderForm({ onSubmit, isSubmitting }: OrderFormProps) {
  const [customerName, setCustomerName] = useState('');
  const [items, setItems] = useState<OrderItemInput[]>([{ menuItemId: '', quantity: 1 }]);
  const [menuItems, setMenuItems] = useState<MenuItem[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getMenuItems()
      .then(setMenuItems)
      .catch(() => setError('Failed to load menu items'));
  }, []);

  const handleAddItem = () => {
    setItems([...items, { menuItemId: '', quantity: 1 }]);
  };

  const handleRemoveItem = (index: number) => {
    if (items.length > 1) {
      setItems(items.filter((_, i) => i !== index));
    }
  };

  const handleItemChange = (index: number, field: keyof OrderItemInput, value: string | number) => {
    const newItems = [...items];
    newItems[index] = { ...newItems[index], [field]: value };
    setItems(newItems);
  };

  const handleQuantityBlur = (index: number) => {
    const newItems = [...items];
    const qty = newItems[index].quantity;
    // Coerce empty string or invalid values to 1 on blur
    if (qty === '' || qty === 0 || (typeof qty === 'number' && isNaN(qty))) {
      newItems[index] = { ...newItems[index], quantity: 1 };
      setItems(newItems);
    }
  };

  // Get list of already selected menu item IDs (excluding a specific index)
  const getSelectedItemIds = (excludeIndex: number) => {
    return items
      .filter((_, i) => i !== excludeIndex)
      .map((item) => item.menuItemId)
      .filter((id) => id !== '');
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    const validItems = items
      .filter((item) => item.menuItemId)
      .map((item) => ({
        menuItemId: item.menuItemId,
        quantity: typeof item.quantity === 'string' ? parseInt(item.quantity) || 1 : item.quantity || 1,
      }))
      .filter((item) => item.quantity > 0);
    if (validItems.length === 0) {
      setError('At least one menu item is required');
      return;
    }

    try {
      await onSubmit({
        customerName: customerName.trim(),
        items: validItems,
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create order');
    }
  };

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-6">
      {error && (
        <div className="p-4 bg-red-50 border border-red-200 rounded-lg text-red-700">
          {error}
        </div>
      )}

      <div className="flex flex-col gap-2">
        <label htmlFor="customerName" className="font-semibold text-slate-800 text-sm">
          Vehicle Description <span className="font-normal text-slate-500">(optional)</span>
        </label>
        <input
          type="text"
          id="customerName"
          value={customerName}
          onChange={(e) => setCustomerName(e.target.value)}
          placeholder="e.g., Red Toyota Camry"
          className="p-3 border border-slate-300 rounded-md text-base transition-all focus:outline-none focus:border-blue-600 focus:ring-2 focus:ring-blue-600/10"
        />
      </div>

      <div className="flex flex-col gap-2">
        <label className="font-semibold text-slate-800 text-sm">Order Items</label>
        <div className="flex flex-col gap-4">
          {items.map((item, index) => (
            <div
              key={index}
              className="flex flex-col gap-3 p-4 bg-slate-50 rounded-lg border border-slate-200 sm:flex-row sm:items-end"
            >
              <div className="flex flex-col gap-1 flex-grow">
                <label className="text-sm text-slate-600">Menu Item</label>
                <select
                  value={item.menuItemId}
                  onChange={(e) => handleItemChange(index, 'menuItemId', e.target.value)}
                  className="p-3 border border-slate-300 rounded-md bg-white w-full"
                >
                  <option value="">Select item...</option>
                  {menuItems
                    .filter((menuItem) =>
                      // Show item if it's the current selection OR not already selected elsewhere
                      menuItem.id === item.menuItemId || !getSelectedItemIds(index).includes(menuItem.id)
                    )
                    .map((menuItem) => (
                      <option key={menuItem.id} value={menuItem.id}>
                        {menuItem.name} - ${menuItem.price.toFixed(2)}
                      </option>
                    ))}
                </select>
              </div>
              <div className="flex gap-3 items-end">
                <div className="flex flex-col gap-1 flex-grow sm:flex-grow-0">
                  <label className="text-sm text-slate-600">Qty</label>
                  <input
                    type="number"
                    min="1"
                    value={item.quantity}
                    onChange={(e) => handleItemChange(index, 'quantity', e.target.value === '' ? '' : parseInt(e.target.value) || '')}
                    onBlur={() => handleQuantityBlur(index)}
                    className="p-3 border border-slate-300 rounded-md w-20"
                  />
                </div>
                <button
                  type="button"
                  onClick={() => handleRemoveItem(index)}
                  disabled={items.length === 1}
                  className="px-4 py-3 bg-red-500 text-white rounded-lg font-medium hover:bg-red-600 transition-all disabled:opacity-50 disabled:cursor-not-allowed shrink-0"
                  aria-label="Remove item"
                >
                  Remove
                </button>
              </div>
            </div>
          ))}
        </div>
        <button
          type="button"
          onClick={handleAddItem}
          className="self-start px-4 py-2 bg-slate-500 text-white rounded-lg font-medium hover:bg-slate-600 transition-all mt-2"
        >
          + Add Item
        </button>
      </div>

      <button
        type="submit"
        disabled={isSubmitting}
        className="self-start px-6 py-3 bg-blue-600 text-white rounded-lg font-semibold hover:bg-blue-700 hover:-translate-y-0.5 transition-all shadow-sm disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {isSubmitting ? 'Creating Order...' : 'Create Order'}
      </button>
    </form>
  );
}
