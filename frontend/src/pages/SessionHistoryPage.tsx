import { useState, useEffect } from 'react';
import { Layout } from '../components/Layout';
import { getSessions, getSessionOrders, compareSessions } from '../api/sessions';
import type { Session, Order, SessionComparisonResponse, ItemBreakdown } from '../types';

export function SessionHistoryPage() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [selectedSessions, setSelectedSessions] = useState<Set<number>>(new Set());
  const [comparison, setComparison] = useState<SessionComparisonResponse | null>(null);
  const [drilldownSession, setDrilldownSession] = useState<Session | null>(null);
  const [drilldownOrders, setDrilldownOrders] = useState<Order[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Date filters
  const [fromDate, setFromDate] = useState('');
  const [toDate, setToDate] = useState('');

  useEffect(() => {
    loadSessions();
  }, [fromDate, toDate]);

  const loadSessions = async () => {
    try {
      setIsLoading(true);
      const data = await getSessions(fromDate || undefined, toDate || undefined);
      setSessions(data);
      setError(null);
    } catch (e) {
      setError('Failed to load sessions');
    } finally {
      setIsLoading(false);
    }
  };

  const toggleSessionSelection = (sessionId: number) => {
    const newSelected = new Set(selectedSessions);
    if (newSelected.has(sessionId)) {
      newSelected.delete(sessionId);
    } else {
      newSelected.add(sessionId);
    }
    setSelectedSessions(newSelected);
  };

  const handleCompare = async () => {
    if (selectedSessions.size < 1) return;
    try {
      const result = await compareSessions(Array.from(selectedSessions));
      setComparison(result);
    } catch (e) {
      setError('Failed to compare sessions');
    }
  };

  const handleDrilldown = async (session: Session) => {
    setDrilldownSession(session);
    try {
      const orders = await getSessionOrders(session.id);
      setDrilldownOrders(orders);
    } catch (e) {
      setError('Failed to load session orders');
    }
  };

  const pieColors = [
    '#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6',
    '#ec4899', '#06b6d4', '#84cc16', '#f97316', '#6366f1',
  ];

  const generatePieGradient = (items: ItemBreakdown[], total: number) => {
    let cumulativePercent = 0;
    const stops = items.map((item, index) => {
      const percent = total > 0 ? (item.revenue / total) * 100 : 0;
      const start = cumulativePercent;
      cumulativePercent += percent;
      const color = pieColors[index % pieColors.length];
      return `${color} ${start}% ${cumulativePercent}%`;
    }).join(', ');
    return items.length > 0 ? `conic-gradient(${stops})` : 'conic-gradient(#e2e8f0 0% 100%)';
  };

  return (
    <Layout>
      <div className="flex flex-col gap-8">
        <div className="flex justify-between items-center">
          <h2 className="text-3xl font-bold text-slate-800">Session History</h2>
          <a
            href="/admin"
            className="px-4 py-2 bg-slate-200 text-slate-700 rounded-md font-medium hover:bg-slate-300 transition-all"
          >
            Back to Admin
          </a>
        </div>

        {error && (
          <div className="p-4 bg-red-50 border border-red-200 rounded-lg text-red-700">{error}</div>
        )}

        {/* Date Filters */}
        <div className="bg-white rounded-xl shadow-md p-6">
          <h3 className="text-lg font-semibold text-slate-800 mb-4">Filter by Date</h3>
          <div className="flex gap-4 flex-wrap items-end">
            <div className="flex flex-col gap-1">
              <label className="text-sm text-slate-600">From</label>
              <input
                type="date"
                value={fromDate}
                onChange={(e) => setFromDate(e.target.value)}
                className="p-2 border border-slate-300 rounded-md"
              />
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-sm text-slate-600">To</label>
              <input
                type="date"
                value={toDate}
                onChange={(e) => setToDate(e.target.value)}
                className="p-2 border border-slate-300 rounded-md"
              />
            </div>
            <button
              onClick={() => { setFromDate(''); setToDate(''); }}
              className="px-4 py-2 bg-slate-100 text-slate-600 rounded-md hover:bg-slate-200"
            >
              Clear
            </button>
          </div>
        </div>

        {/* Sessions List */}
        <div className="bg-white rounded-xl shadow-md p-6">
          <div className="flex justify-between items-center mb-4">
            <h3 className="text-lg font-semibold text-slate-800">Sessions</h3>
            {selectedSessions.size > 0 && (
              <button
                onClick={handleCompare}
                className="px-4 py-2 bg-blue-600 text-white rounded-md font-medium hover:bg-blue-700"
              >
                Compare Selected ({selectedSessions.size})
              </button>
            )}
          </div>

          {isLoading ? (
            <p className="text-slate-500">Loading sessions...</p>
          ) : sessions.length === 0 ? (
            <p className="text-slate-500">No sessions found.</p>
          ) : (
            <div className="space-y-2">
              {sessions.map((session) => (
                <div
                  key={session.id}
                  className={`flex items-center justify-between p-4 rounded-lg border transition-all ${
                    selectedSessions.has(session.id)
                      ? 'bg-blue-50 border-blue-300'
                      : 'bg-slate-50 border-slate-200 hover:bg-slate-100'
                  }`}
                >
                  <div className="flex items-center gap-4">
                    <input
                      type="checkbox"
                      checked={selectedSessions.has(session.id)}
                      onChange={() => toggleSessionSelection(session.id)}
                      className="w-5 h-5 rounded border-slate-300"
                    />
                    <div>
                      <h4 className="font-semibold text-slate-800">{session.eventName}</h4>
                      <p className="text-sm text-slate-500">
                        {new Date(session.startedAt).toLocaleDateString()}
                        {session.closedAt && ` - Closed ${new Date(session.closedAt).toLocaleTimeString()}`}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-6">
                    <div className="text-right">
                      <p className="font-semibold text-slate-700">
                        {session.finalOrderCount ?? session.currentOrderCount ?? 0} orders
                      </p>
                      <p className="text-emerald-600 font-semibold">
                        ${(session.finalRevenue ?? session.currentRevenue ?? 0).toFixed(2)}
                      </p>
                    </div>
                    <span
                      className={`px-3 py-1 rounded-full text-sm font-medium ${
                        session.status === 'ACTIVE'
                          ? 'bg-emerald-100 text-emerald-800'
                          : 'bg-slate-100 text-slate-600'
                      }`}
                    >
                      {session.status}
                    </span>
                    <button
                      onClick={() => handleDrilldown(session)}
                      className="px-3 py-1 bg-slate-200 text-slate-700 rounded-md text-sm hover:bg-slate-300"
                    >
                      View Orders
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Comparison Results */}
        {comparison && (
          <div className="bg-white rounded-xl shadow-md p-6">
            <div className="flex justify-between items-center mb-4">
              <h3 className="text-lg font-semibold text-slate-800">Comparison Results</h3>
              <button
                onClick={() => setComparison(null)}
                className="text-slate-500 hover:text-slate-700"
              >
                Clear
              </button>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {/* Summary Stats */}
              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div className="bg-slate-50 p-4 rounded-lg text-center">
                    <p className="text-3xl font-bold text-blue-600">{comparison.totalOrders}</p>
                    <p className="text-sm text-slate-500">Total Orders</p>
                  </div>
                  <div className="bg-slate-50 p-4 rounded-lg text-center">
                    <p className="text-3xl font-bold text-emerald-600">
                      ${comparison.totalRevenue.toFixed(2)}
                    </p>
                    <p className="text-sm text-slate-500">Total Revenue</p>
                  </div>
                </div>

                {/* Session Breakdown */}
                <div className="space-y-2">
                  <h4 className="font-medium text-slate-700">By Session</h4>
                  {comparison.sessions.map((s) => (
                    <div key={s.sessionId} className="flex justify-between p-2 bg-slate-50 rounded">
                      <span className="text-slate-600">{s.eventName}</span>
                      <span className="font-medium">{s.orderCount} orders / ${s.revenue.toFixed(2)}</span>
                    </div>
                  ))}
                </div>
              </div>

              {/* Pie Chart */}
              <div className="flex flex-col items-center gap-4">
                <div
                  className="w-48 h-48 rounded-full shadow-inner"
                  style={{ background: generatePieGradient(comparison.itemBreakdown, comparison.totalRevenue) }}
                />
                <div className="w-full space-y-2">
                  {comparison.itemBreakdown.map((item, index) => (
                    <div key={item.itemName} className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <div
                          className="w-4 h-4 rounded-sm"
                          style={{ backgroundColor: pieColors[index % pieColors.length] }}
                        />
                        <span className="text-sm text-slate-600">{item.itemName}</span>
                      </div>
                      <div className="text-sm">
                        <span className="text-slate-500">{item.quantity} sold</span>
                        <span className="ml-2 font-medium text-emerald-600">${item.revenue.toFixed(2)}</span>
                        <span className="ml-2 text-slate-400">({item.percent.toFixed(0)}%)</span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Session Drilldown Modal */}
        {drilldownSession && (
          <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
            <div className="bg-white rounded-xl max-w-4xl w-full max-h-[80vh] overflow-hidden flex flex-col">
              <div className="p-6 border-b border-slate-200 flex justify-between items-center">
                <div>
                  <h3 className="text-xl font-semibold text-slate-800">{drilldownSession.eventName}</h3>
                  <p className="text-sm text-slate-500">
                    {new Date(drilldownSession.startedAt).toLocaleDateString()} - 
                    {drilldownSession.finalOrderCount ?? drilldownSession.currentOrderCount ?? 0} orders, 
                    ${(drilldownSession.finalRevenue ?? drilldownSession.currentRevenue ?? 0).toFixed(2)} revenue
                  </p>
                </div>
                <button
                  onClick={() => { setDrilldownSession(null); setDrilldownOrders([]); }}
                  className="text-slate-500 hover:text-slate-700 text-2xl"
                >
                  ×
                </button>
              </div>
              <div className="flex-1 overflow-auto p-6">
                {drilldownOrders.length === 0 ? (
                  <p className="text-slate-500 text-center py-8">No orders in this session.</p>
                ) : (
                  <div className="space-y-3">
                    {drilldownOrders.map((order) => (
                      <div
                        key={order.id}
                        className="p-4 bg-slate-50 rounded-lg border border-slate-200"
                      >
                        <div className="flex justify-between items-start">
                          <div>
                            <h4 className="font-semibold text-slate-800">
                              Order #{order.dailyOrderNumber}
                            </h4>
                            {order.customerName && (
                              <p className="text-sm text-slate-500">{order.customerName}</p>
                            )}
                          </div>
                          <div className="text-right">
                            <span
                              className={`px-2 py-1 rounded text-xs font-medium ${
                                order.status === 'completed'
                                  ? 'bg-emerald-100 text-emerald-800'
                                  : order.status === 'in-progress'
                                  ? 'bg-blue-100 text-blue-800'
                                  : 'bg-amber-100 text-amber-800'
                              }`}
                            >
                              {order.status}
                            </span>
                            <p className="text-lg font-semibold text-emerald-600 mt-1">
                              ${order.total.toFixed(2)}
                            </p>
                          </div>
                        </div>
                        <div className="mt-2 text-sm text-slate-600">
                          {order.items.map((item) => (
                            <span key={item.id} className="mr-3">
                              {item.quantity}x {item.menuItemName}
                            </span>
                          ))}
                        </div>
                        <p className="text-xs text-slate-400 mt-2">
                          {new Date(order.created_at).toLocaleString()}
                        </p>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          </div>
        )}
      </div>
    </Layout>
  );
}
