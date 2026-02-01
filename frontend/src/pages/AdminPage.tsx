import { useState, useEffect } from 'react';
import { Layout } from '../components/Layout';
import { useOrders } from '../hooks/useOrders';
import { getMenuItems, createMenuItem, deleteMenuItem, updateMenuItemsOrder, updateMenuItem } from '../api/menu';
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
  const [ordersByItemExpanded, setOrdersByItemExpanded] = useState(false);
  const [purgeExpanded, setPurgeExpanded] = useState(false);

  // Drilldown state
  const [selectedItemId, setSelectedItemId] = useState<string | null>(null);

  // New menu item form
  const [newItemName, setNewItemName] = useState('');
  const [newItemPrice, setNewItemPrice] = useState('');

  // Edit menu item state
  const [editingItemId, setEditingItemId] = useState<string | null>(null);
  const [editingPrice, setEditingPrice] = useState('');

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

  const handleStartEdit = (item: MenuItem) => {
    setEditingItemId(item.id);
    setEditingPrice(item.price.toFixed(2));
  };

  const handleCancelEdit = () => {
    setEditingItemId(null);
    setEditingPrice('');
  };

  const handleSavePrice = async (id: string) => {
    const price = parseFloat(editingPrice);
    if (isNaN(price) || price < 0) {
      setError('Invalid price');
      return;
    }

    try {
      const updated = await updateMenuItem(id, { price });
      setMenuItems(menuItems.map((item) => (item.id === id ? updated : item)));
      setEditingItemId(null);
      setEditingPrice('');
    } catch (e) {
      setError('Failed to update menu item price');
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

  // Calculate revenue and quantity per menu item
  const itemRevenueStats = menuItems.map((menuItem) => {
    const ordersWithItem = orders.filter((order) =>
      order.items.some((item) => item.menuItemId === menuItem.id || item.menu_item_id === menuItem.id)
    );
    const totalQuantity = orders.reduce((sum, order) => {
      const item = order.items.find((i) => i.menuItemId === menuItem.id || i.menu_item_id === menuItem.id);
      return sum + (item?.quantity || 0);
    }, 0);
    const revenue = totalQuantity * menuItem.price;
    return {
      menuItem,
      orderCount: ordersWithItem.length,
      totalQuantity,
      revenue,
      orders: ordersWithItem,
    };
  }).filter((stat) => stat.totalQuantity > 0); // Only show items that have been sold

  const totalItemRevenue = itemRevenueStats.reduce((sum, stat) => sum + stat.revenue, 0);

  // Generate pie chart gradient
  const pieColors = [
    '#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6',
    '#ec4899', '#06b6d4', '#84cc16', '#f97316', '#6366f1',
  ];
  let cumulativePercent = 0;
  const pieGradientStops = itemRevenueStats.map((stat, index) => {
    const percent = totalItemRevenue > 0 ? (stat.revenue / totalItemRevenue) * 100 : 0;
    const start = cumulativePercent;
    cumulativePercent += percent;
    const color = pieColors[index % pieColors.length];
    return `${color} ${start}% ${cumulativePercent}%`;
  }).join(', ');
  const pieGradient = itemRevenueStats.length > 0
    ? `conic-gradient(${pieGradientStops})`
    : 'conic-gradient(#e2e8f0 0% 100%)';

  const selectedItemStats = itemRevenueStats.find((s) => s.menuItem.id === selectedItemId);

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

        {/* Revenue by Item Section */}
        <div className="bg-white rounded-xl shadow-md overflow-hidden">
          <div
            className="flex justify-between items-center p-6 cursor-pointer hover:bg-slate-50 transition-colors"
            onClick={() => setOrdersByItemExpanded(!ordersByItemExpanded)}
          >
            <h3 className="text-xl font-semibold text-slate-800">Revenue by Item</h3>
            <button className="text-slate-500 text-xl">{ordersByItemExpanded ? '▼' : '▶'}</button>
          </div>
          {ordersByItemExpanded && (
            <div className="px-6 pb-6">
              {itemRevenueStats.length === 0 ? (
                <p className="text-slate-500 text-center py-4">No items sold yet.</p>
              ) : (
                <div className="flex flex-col lg:flex-row gap-6">
                  {/* Pie Chart */}
                  <div className="flex flex-col items-center gap-4">
                    <div
                      className="w-48 h-48 rounded-full shadow-inner"
                      style={{ background: pieGradient }}
                    />
                    <div className="text-center">
                      <div className="text-sm text-slate-500">Total Revenue</div>
                      <div className="text-xl font-bold text-blue-600">${totalItemRevenue.toFixed(2)}</div>
                    </div>
                  </div>

                  {/* Item List */}
                  <div className="flex-1 flex flex-col gap-2">
                    {itemRevenueStats.map((stat, index) => (
                      <div key={stat.menuItem.id}>
                        <button
                          onClick={() => setSelectedItemId(selectedItemId === stat.menuItem.id ? null : stat.menuItem.id)}
                          className={`w-full flex justify-between items-center p-3 rounded-lg border transition-all ${
                            selectedItemId === stat.menuItem.id
                              ? 'bg-blue-50 border-blue-300'
                              : 'bg-slate-50 border-slate-200 hover:bg-slate-100'
                          }`}
                        >
                          <div className="flex items-center gap-3">
                            <div
                              className="w-4 h-4 rounded-sm shrink-0"
                              style={{ backgroundColor: pieColors[index % pieColors.length] }}
                            />
                            <span className="font-medium text-slate-800">{stat.menuItem.name}</span>
                          </div>
                          <div className="flex items-center gap-4 text-sm">
                            <span className="text-slate-500">{stat.totalQuantity} sold</span>
                            <span className="font-semibold text-emerald-600">${stat.revenue.toFixed(2)}</span>
                            <span className="text-slate-400 text-xs">
                              ({totalItemRevenue > 0 ? ((stat.revenue / totalItemRevenue) * 100).toFixed(0) : 0}%)
                            </span>
                            <span className="text-slate-400">{selectedItemId === stat.menuItem.id ? '▼' : '▶'}</span>
                          </div>
                        </button>
                        {selectedItemId === stat.menuItem.id && selectedItemStats && (
                          <div className="mt-2 ml-6 border-l-2 border-blue-200 pl-4">
                            <div className="flex flex-col gap-2 py-2">
                              {selectedItemStats.orders.map((order) => {
                                const item = order.items.find(
                                  (i) => i.menuItemId === stat.menuItem.id || i.menu_item_id === stat.menuItem.id
                                );
                                return (
                                  <a
                                    key={order.id}
                                    href={`/orders/${order.id}`}
                                    className="flex justify-between items-center p-2 bg-white rounded border border-slate-200 hover:border-blue-300 hover:bg-blue-50 transition-all text-sm"
                                  >
                                    <span className="font-medium text-blue-600">
                                      #{order.dailyOrderNumber || order.id}
                                    </span>
                                    <span className="text-slate-600">
                                      {item?.quantity}x - {order.customerName || order.vehicle_description || 'No vehicle'}
                                    </span>
                                  </a>
                                );
                              })}
                            </div>
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              )}
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
                            {editingItemId === item.id ? (
                              <div className="flex items-center gap-2 mt-1">
                                <span className="text-slate-600">$</span>
                                <input
                                  type="number"
                                  step="0.01"
                                  min="0"
                                  value={editingPrice}
                                  onChange={(e) => setEditingPrice(e.target.value)}
                                  className="p-1 border border-slate-300 rounded w-20 text-sm"
                                  autoFocus
                                />
                                <button
                                  onClick={() => handleSavePrice(item.id)}
                                  className="px-2 py-1 bg-emerald-500 text-white rounded text-xs font-medium hover:bg-emerald-600"
                                >
                                  Save
                                </button>
                                <button
                                  onClick={handleCancelEdit}
                                  className="px-2 py-1 bg-slate-400 text-white rounded text-xs font-medium hover:bg-slate-500"
                                >
                                  Cancel
                                </button>
                              </div>
                            ) : (
                              <button
                                onClick={() => handleStartEdit(item)}
                                className="text-blue-600 font-semibold hover:underline"
                                title="Click to edit price"
                              >
                                ${item.price.toFixed(2)}
                              </button>
                            )}
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
