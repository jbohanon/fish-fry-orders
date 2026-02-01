export type OrderStatus = 'new' | 'in-progress' | 'completed';

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

export interface PurgeOrdersRequest {
  scope: 'today' | 'all';
}

export interface PurgeOrdersResponse {
  deleted: number;
  scope: string;
}

export interface WebSocketMessage {
  type: 'order_new' | 'order_update' | 'stats_update';
  order?: Order;
  stats?: Stats;
}

export interface ChatMessage {
  id: string;
  order_id: number;
  content: string;
  sender_role: string;
  created_at: string;
}
