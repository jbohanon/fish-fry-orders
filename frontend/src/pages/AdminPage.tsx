import { useState, useEffect } from 'react';
import { Layout } from '../components/Layout';
import { useOrders } from '../hooks/useOrders';
import { getMenuItems, createMenuItem, deleteMenuItem, updateMenuItemsOrder } from '../api/menu';
import { purgeOrders } from '../api/orders';
import type { MenuItem } from '../types';

export function AdminPage() {
  const { orders, stats } = useOrders();
  const [menuItems, setMenuItems] = useState<MenuItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Collapsible sections
  const [menuExpanded, setMenuExpanded] = useState(true);
  const [statsExpanded, setStatsExpanded] = useState(true);
  const [purgeExpanded, setPurgeExpanded] = useState(false);

  // New menu item form
  const [newItemName, setNewItemName] = useState('');
  const [newItemPrice, setNewItemPrice] = useState('');

  useEffect(() => {
    loadMenuItems();
  }, []);

  const loadMenuItems = async () => {
    try {
      setIsLoading(true);
      const items = await getMenuItems();
      setMenuItems(items);
    } catch (e) {
      setError('Failed to load menu items');
    } finally {
      setIsLoading(false);
    }
  };

  const handleAddMenuItem = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newItemName.trim() || !newItemPrice) return;

    try {
      const newItem = await createMenuItem({
        name: newItemName.trim(),
        price: parseFloat(newItemPrice),
        is_active: true,
      });
      setMenuItems([...menuItems, newItem]);
      setNewItemName('');
      setNewItemPrice('');
    } catch (e) {
      setError('Failed to create menu item');
    }
  };

  const handleDeleteMenuItem = async (id: string) => {
    if (!confirm('Are you sure you want to delete this menu item?')) return;

    try {
      await deleteMenuItem(id);
      setMenuItems(menuItems.filter((item) => item.id !== id));
    } catch (e) {
      setError('Failed to delete menu item');
    }
  };

  const handleMoveItem = async (index: number, direction: 'up' | 'down') => {
    const newIndex = direction === 'up' ? index - 1 : index + 1;
    if (newIndex < 0 || newIndex >= menuItems.length) return;

    // Swap items in the array
    const newItems = [...menuItems];
    [newItems[index], newItems[newIndex]] = [newItems[newIndex], newItems[index]];
    setMenuItems(newItems);

    // Create order map with new positions
    const itemOrders: Record<string, number> = {};
    newItems.forEach((item, i) => {
      itemOrders[item.id] = i + 1;
    });

    try {
      await updateMenuItemsOrder(itemOrders);
    } catch (e) {
      // Revert on error
      setMenuItems(menuItems);
      setError('Failed to reorder menu items');
    }
  };

  const handlePurgeOrders = async (scope: 'today' | 'all') => {
    const confirmMsg = scope === 'all'
      ? 'Are you sure you want to delete ALL orders? This cannot be undone.'
      : "Are you sure you want to delete today's orders?";

    if (!confirm(confirmMsg)) return;

    try {
      const result = await purgeOrders({ scope });
      alert(`Successfully deleted ${result.deleted} orders.`);
      window.location.reload();
    } catch (e) {
      setError('Failed to purge orders');
    }
  };

  // Calculate stats from orders if not from WebSocket
  const ordersToday = orders.filter((o) => {
    const today = new Date();
    const orderDate = new Date(o.created_at);
    return (
      orderDate.getDate() === today.getDate() &&
      orderDate.getMonth() === today.getMonth() &&
      orderDate.getFullYear() === today.getFullYear()
    );
  }).length;

  const totalRevenue = orders.reduce((sum, o) => sum + o.total, 0);

  const displayStats = stats || {
    totalOrders: orders.length,
    ordersToday,
    revenue: totalRevenue,
  };

  return (
    <Layout>
      <div className="flex flex-col gap-8">
        <h2 className="text-3xl font-bold text-slate-800">Admin Dashboard</h2>

        {error && (
          <div className="p-4 bg-red-50 border border-red-200 rounded-lg text-red-700">{error}</div>
        )}

        {/* Statistics Section */}
        <div className="bg-white rounded-xl shadow-md overflow-hidden">
          <div
            className="flex justify-between items-center p-6 cursor-pointer hover:bg-slate-50 transition-colors"
            onClick={() => setStatsExpanded(!statsExpanded)}
          >
            <h3 className="text-xl font-semibold text-slate-800">Statistics</h3>
            <button className="text-slate-500 text-xl">{statsExpanded ? '▼' : '▶'}</button>
          </div>
          {statsExpanded && (
            <div className="px-6 pb-6">
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div className="bg-slate-50 p-6 rounded-lg border border-slate-200 text-center">
                  <h4 className="text-sm text-slate-500 uppercase tracking-wide mb-2">Total Orders</h4>
                  <div className="text-3xl font-bold text-blue-600">{displayStats.totalOrders}</div>
                </div>
                <div className="bg-slate-50 p-6 rounded-lg border border-slate-200 text-center">
                  <h4 className="text-sm text-slate-500 uppercase tracking-wide mb-2">Orders Today</h4>
                  <div className="text-3xl font-bold text-blue-600">{displayStats.ordersToday}</div>
                </div>
                <div className="bg-slate-50 p-6 rounded-lg border border-slate-200 text-center">
                  <h4 className="text-sm text-slate-500 uppercase tracking-wide mb-2">Total Revenue</h4>
                  <div className="text-3xl font-bold text-blue-600">${displayStats.revenue.toFixed(2)}</div>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Menu Items Section */}
        <div className="bg-white rounded-xl shadow-md overflow-hidden">
          <div
            className="flex justify-between items-center p-6 cursor-pointer hover:bg-slate-50 transition-colors"
            onClick={() => setMenuExpanded(!menuExpanded)}
          >
            <h3 className="text-xl font-semibold text-slate-800">Menu Items</h3>
            <button className="text-slate-500 text-xl">{menuExpanded ? '▼' : '▶'}</button>
          </div>
          {menuExpanded && (
            <div className="px-6 pb-6">
              {isLoading ? (
                <div className="text-slate-500">Loading menu items...</div>
              ) : (
                <>
                  <div className="flex flex-col gap-3 mb-6">
                    {menuItems.map((item, index) => (
                      <div
                        key={item.id}
                        className="flex justify-between items-center p-4 bg-slate-50 rounded-lg border border-slate-200"
                      >
                        <div className="flex items-center gap-3">
                          <div className="flex flex-col gap-1">
                            <button
                              onClick={() => handleMoveItem(index, 'up')}
                              disabled={index === 0}
                              className="px-2 py-0.5 bg-slate-200 text-slate-700 rounded text-xs font-bold hover:bg-slate-300 transition-all disabled:opacity-30 disabled:cursor-not-allowed"
                              title="Move up"
                            >
                              ▲
                            </button>
                            <button
                              onClick={() => handleMoveItem(index, 'down')}
                              disabled={index === menuItems.length - 1}
                              className="px-2 py-0.5 bg-slate-200 text-slate-700 rounded text-xs font-bold hover:bg-slate-300 transition-all disabled:opacity-30 disabled:cursor-not-allowed"
                              title="Move down"
                            >
                              ▼
                            </button>
                          </div>
                          <div>
                            <h4 className="font-semibold text-slate-800">{item.name}</h4>
                            <span className="text-blue-600 font-semibold">${item.price.toFixed(2)}</span>
                          </div>
                        </div>
                        <button
                          onClick={() => handleDeleteMenuItem(item.id)}
                          className="px-3 py-1.5 bg-red-500 text-white rounded-md text-sm font-medium hover:bg-red-600 transition-all"
                        >
                          Delete
                        </button>
                      </div>
                    ))}
                  </div>

                  {/* Add new menu item form */}
                  <form onSubmit={handleAddMenuItem} className="flex gap-3 items-end flex-wrap">
                    <div className="flex flex-col gap-1">
                      <label className="text-sm text-slate-600">Item Name</label>
                      <input
                        type="text"
                        value={newItemName}
                        onChange={(e) => setNewItemName(e.target.value)}
                        placeholder="e.g., Fish Dinner"
                        className="p-2 border border-slate-300 rounded-md"
                      />
                    </div>
                    <div className="flex flex-col gap-1">
                      <label className="text-sm text-slate-600">Price</label>
                      <input
                        type="number"
                        step="0.01"
                        min="0"
                        value={newItemPrice}
                        onChange={(e) => setNewItemPrice(e.target.value)}
                        placeholder="0.00"
                        className="p-2 border border-slate-300 rounded-md w-24"
                      />
                    </div>
                    <button
                      type="submit"
                      className="px-4 py-2 bg-blue-600 text-white rounded-md font-medium hover:bg-blue-700 transition-all"
                    >
                      Add Item
                    </button>
                  </form>
                </>
              )}
            </div>
          )}
        </div>

        {/* Purge Orders Section */}
        <div className="bg-white rounded-xl shadow-md overflow-hidden">
          <div
            className="flex justify-between items-center p-6 cursor-pointer hover:bg-slate-50 transition-colors"
            onClick={() => setPurgeExpanded(!purgeExpanded)}
          >
            <h3 className="text-xl font-semibold text-slate-800">Danger Zone</h3>
            <button className="text-slate-500 text-xl">{purgeExpanded ? '▼' : '▶'}</button>
          </div>
          {purgeExpanded && (
            <div className="px-6 pb-6">
              <p className="text-slate-600 mb-4">
                These actions are destructive and cannot be undone. Please be careful.
              </p>
              <div className="flex gap-4 flex-wrap">
                <button
                  onClick={() => handlePurgeOrders('today')}
                  className="px-4 py-2 bg-amber-500 text-white rounded-md font-medium hover:bg-amber-600 transition-all"
                >
                  Purge Today's Orders
                </button>
                <button
                  onClick={() => handlePurgeOrders('all')}
                  className="px-4 py-2 bg-red-500 text-white rounded-md font-medium hover:bg-red-600 transition-all"
                >
                  Purge All Orders
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </Layout>
  );
}
