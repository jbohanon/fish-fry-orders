export type OrderStatus = 'new' | 'in-progress' | 'completed';
export type SessionStatus = 'ACTIVE' | 'CLOSED';

export interface OrderItem {
  id: string;
  menu_item_id: string;
  menuItemId: string;
  menuItemName: string;
  price: number;
  quantity: number;
}

export interface Order {
  id: number;
  dailyOrderNumber: number;
  vehicle_description: string;
  customerName: string;
  status: OrderStatus;
  items: OrderItem[];
  total: number;
  created_at: string;
  updated_at: string;
}

export interface MenuItem {
  id: string;
  name: string;
  price: number;
  is_active: boolean;
}

export interface Stats {
  totalOrders: number;
  ordersToday: number;
  revenue: number;
}

export interface User {
  role: 'worker' | 'admin';
}

// Session types
export interface Session {
  id: number;
  eventName: string;
  startedAt: string;
  expiresAt: string;
  closedAt?: string;
  status: SessionStatus;
  finalOrderCount?: number;
  finalRevenue?: number;
  notes?: string;
  currentOrderCount?: number;
  currentRevenue?: number;
}

export interface SessionResponse {
  active: boolean;
  session?: Session;
}

export interface CreateSessionRequest {
  eventName?: string;
  expiresAt?: string;
  notes?: string;
}

export interface UpdateSessionRequest {
  eventName?: string;
  expiresAt?: string;
  notes?: string;
}

export interface SessionComparisonItem {
  sessionId: number;
  eventName: string;
  startedAt: string;
  orderCount: number;
  revenue: number;
}

export interface ItemBreakdown {
  itemName: string;
  quantity: number;
  revenue: number;
  percent: number;
}

export interface SessionComparisonResponse {
  sessions: SessionComparisonItem[];
  totalOrders: number;
  totalRevenue: number;
  itemBreakdown: ItemBreakdown[];
}

export interface CreateOrderRequest {
  customerName: string;
  items: {
    menuItemId: string;
    quantity: number;
  }[];
}

export interface UpdateStatusRequest {
  status: OrderStatus;
}

export interface UpdateOrderRequest {
  customerName: string;
  items: {
    menuItemId: string;
    quantity: number;
  }[];
}

export interface PurgeOrdersRequest {
  scope: 'today' | 'all';
}

export interface PurgeOrdersResponse {
  deleted: number;
  scope: string;
}

export interface WebSocketMessage {
  type: 'order_new' | 'order_update' | 'stats_update' | 'session_update' | 'session_closed';
  order?: Order;
  stats?: Stats;
  session?: Session;
  active?: boolean;
}

export interface ChatMessage {
  id: string;
  order_id: number;
  content: string;
  sender_role: string;
  created_at: string;
}
