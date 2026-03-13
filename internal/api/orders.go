package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.nonahob.net/jacob/fish-fry-orders/internal/auth"
	"git.nonahob.net/jacob/fish-fry-orders/internal/database"
	"git.nonahob.net/jacob/fish-fry-orders/internal/logger"
	"git.nonahob.net/jacob/fish-fry-orders/internal/types"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

type OrderHandler struct {
	repo           database.Repository
	authService    *auth.Service
	clients        map[*websocket.Conn]bool
	clientsLock    sync.RWMutex
	allowedOrigins []string
	upgrader       websocket.Upgrader
}

func NewOrderHandler(repo database.Repository, authService *auth.Service, allowedOrigins []string) *OrderHandler {
	h := &OrderHandler{
		repo:           repo,
		authService:    authService,
		clients:        make(map[*websocket.Conn]bool),
		allowedOrigins: allowedOrigins,
	}
	h.upgrader = websocket.Upgrader{
		CheckOrigin: h.checkOrigin,
	}
	return h
}

func (h *OrderHandler) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // Allow requests without Origin header (same-origin)
	}
	for _, allowed := range h.allowedOrigins {
		if origin == allowed {
			return true
		}
	}
	logger.Warn("WebSocket connection rejected: origin not allowed", "origin", origin, "allowed", h.allowedOrigins)
	return false
}

// CreateOrderRequest represents the request to create an order
type CreateOrderRequest struct {
	VehicleDescription string                   `json:"vehicle_description"`
	CustomerName       string                   `json:"customerName"` // Frontend uses this
	Items              []CreateOrderItemRequest `json:"items"`
}

type CreateOrderItemRequest struct {
	MenuItemID string `json:"menu_item_id"`
	MenuItemId string `json:"menuItemId"` // Frontend uses camelCase
	Quantity   int32  `json:"quantity"`
}

// UpdateOrderRequest represents the request to edit an order.
type UpdateOrderRequest struct {
	VehicleDescription string                   `json:"vehicle_description"`
	CustomerName       string                   `json:"customerName"`
	Items              []CreateOrderItemRequest `json:"items"`
}

// OrderResponse represents an order in the API response
type OrderResponse struct {
	ID                 int                 `json:"id"`
	DailyOrderNumber   int                 `json:"dailyOrderNumber"`
	VehicleDescription string              `json:"vehicle_description"`
	CustomerName       string              `json:"customerName"` // Alias for vehicle_description for frontend
	Status             string              `json:"status"`
	Items              []OrderItemResponse `json:"items"`
	Total              float64             `json:"total"`
	CreatedAt          string              `json:"created_at"`
	UpdatedAt          string              `json:"updated_at"`
}

type OrderItemResponse struct {
	ID           string  `json:"id"`
	MenuItemID   string  `json:"menu_item_id"`
	MenuItemId   string  `json:"menuItemId"` // Also support camelCase for frontend
	MenuItemName string  `json:"menuItemName"`
	Price        float64 `json:"price"`
	Quantity     int32   `json:"quantity"`
}

// CreateOrder handles POST /api/orders
func (h *OrderHandler) CreateOrder(c echo.Context) error {
	var req CreateOrderRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Accept either vehicle_description or customerName (both optional)
	vehicleDesc := req.VehicleDescription
	if vehicleDesc == "" {
		vehicleDesc = req.CustomerName
	}
	if len(req.Items) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "at least one item is required")
	}

	ctx := c.Request().Context()

	// Get or create active session
	session, err := h.repo.GetOrCreateActiveSession(ctx)
	if err != nil {
		logger.ErrorWithErr("Failed to get or create session", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get or create session")
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		return echo.NewHTTPError(http.StatusConflict, "Session has expired. Please ask an admin to extend the session or start a new one.")
	}

	// Build order items with captured prices (validate menu items first)
	orderItems := make([]*types.DBOrderItem, 0, len(req.Items))
	for _, itemReq := range req.Items {
		menuItemID := itemReq.MenuItemID
		if menuItemID == "" {
			menuItemID = itemReq.MenuItemId // Use camelCase if snake_case is empty
		}

		// Look up menu item to capture name and price
		menuItem, err := h.repo.GetMenuItemByID(ctx, menuItemID)
		if err != nil {
			logger.ErrorWithErr("Failed to get menu item", err, "menu_item_id", menuItemID)
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get menu item")
		}
		if menuItem == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "menu item not found: "+menuItemID)
		}

		orderItems = append(orderItems, &types.DBOrderItem{
			MenuItemID: menuItemID,
			ItemName:   menuItem.Name,  // Capture at order time
			UnitPrice:  menuItem.Price, // Capture at order time
			Quantity:   itemReq.Quantity,
		})
	}

	// Create order with items in a single transaction
	// This atomically assigns the daily order number and creates all items
	order := &types.DBOrder{
		SessionID:          session.ID,
		VehicleDescription: vehicleDesc,
		Status:             "NEW",
	}

	if err := h.repo.CreateOrderWithItems(ctx, order, orderItems); err != nil {
		logger.ErrorWithErr("Failed to create order with items", err,
			"session_id", session.ID,
			"vehicle_description", vehicleDesc,
			"items_count", len(req.Items),
		)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create order")
	}

	logger.Info("Order created successfully",
		"order_id", order.ID,
		"session_id", session.ID,
		"daily_order_number", order.DailyOrderNumber,
		"vehicle_description", vehicleDesc,
	)

	// Fetch the complete order with items for response
	createdOrder, err := h.repo.GetOrderByID(ctx, order.ID)
	if err != nil || createdOrder == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get created order")
	}

	// Broadcast new order to WebSocket clients
	h.BroadcastNewOrder(ctx, createdOrder)

	// Broadcast stats update
	h.BroadcastStatsUpdate(ctx)

	return h.getOrderResponse(c, order.ID)
}

// GetOrders handles GET /api/orders
func (h *OrderHandler) GetOrders(c echo.Context) error {
	orders, err := h.repo.GetOrders(c.Request().Context())
	if err != nil {
		logger.ErrorWithErr("Failed to get orders", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get orders")
	}

	responses := make([]OrderResponse, 0, len(orders))
	for _, order := range orders {
		// Get order items (now includes captured name and price)
		items, err := h.repo.GetOrderItems(c.Request().Context(), order.ID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get order items")
		}

		var total float64
		itemResponses := make([]OrderItemResponse, 0, len(items))
		for _, item := range items {
			// Use captured price and name from order time
			total += item.UnitPrice * float64(item.Quantity)
			itemResponses = append(itemResponses, OrderItemResponse{
				ID:           item.ID,
				MenuItemID:   item.MenuItemID,
				MenuItemId:   item.MenuItemID,
				MenuItemName: item.ItemName,
				Price:        item.UnitPrice,
				Quantity:     item.Quantity,
			})
		}

		responses = append(responses, OrderResponse{
			ID:                 order.ID,
			DailyOrderNumber:   order.DailyOrderNumber,
			VehicleDescription: order.VehicleDescription,
			CustomerName:       order.VehicleDescription, // Alias for frontend
			Status:             normalizeStatus(order.Status),
			Items:              itemResponses,
			Total:              total,
			CreatedAt:          order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:          order.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return c.JSON(http.StatusOK, responses)
}

// GetOrder handles GET /api/orders/:id
func (h *OrderHandler) GetOrder(c echo.Context) error {
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid order ID")
	}
	return h.getOrderResponse(c, orderID)
}

func (h *OrderHandler) getOrderResponse(c echo.Context, orderID int) error {
	order, err := h.repo.GetOrderByID(c.Request().Context(), orderID)
	if err != nil {
		logger.ErrorWithErr("Failed to get order", err, "order_id", orderID)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get order")
	}
	if order == nil {
		logger.Warn("Order not found", "order_id", orderID)
		return echo.NewHTTPError(http.StatusNotFound, "order not found")
	}

	// Get order items (now includes captured name and price)
	items, err := h.repo.GetOrderItems(c.Request().Context(), orderID)
	if err != nil {
		logger.ErrorWithErr("Failed to get order items", err, "order_id", orderID)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get order items")
	}

	var total float64
	itemResponses := make([]OrderItemResponse, 0, len(items))
	for _, item := range items {
		// Use captured price and name from order time
		total += item.UnitPrice * float64(item.Quantity)
		itemResponses = append(itemResponses, OrderItemResponse{
			ID:           item.ID,
			MenuItemID:   item.MenuItemID,
			MenuItemId:   item.MenuItemID, // Also set camelCase version
			MenuItemName: item.ItemName,
			Price:        item.UnitPrice,
			Quantity:     item.Quantity,
		})
	}

	return c.JSON(http.StatusOK, OrderResponse{
		ID:                 order.ID,
		DailyOrderNumber:   order.DailyOrderNumber,
		VehicleDescription: order.VehicleDescription,
		CustomerName:       order.VehicleDescription, // Alias for frontend
		Status:             normalizeStatus(order.Status),
		Items:              itemResponses,
		Total:              total,
		CreatedAt:          order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:          order.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// UpdateOrder handles PUT /api/orders/:id
func (h *OrderHandler) UpdateOrder(c echo.Context) error {
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid order ID")
	}

	var req UpdateOrderRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	vehicleDesc := req.VehicleDescription
	if vehicleDesc == "" {
		vehicleDesc = req.CustomerName
	}
	if len(req.Items) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "at least one item is required")
	}

	order, err := h.repo.GetOrderByID(c.Request().Context(), orderID)
	if err != nil {
		logger.ErrorWithErr("Failed to get order for update", err, "order_id", orderID)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get order")
	}
	if order == nil {
		return echo.NewHTTPError(http.StatusNotFound, "order not found")
	}

	user, err := h.authService.GetUser(c.Request())
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	if (order.Status == "IN_PROGRESS" || order.Status == "COMPLETED") && user.Role != auth.RoleAdmin {
		return echo.NewHTTPError(http.StatusUnauthorized, "only admins can edit in-progress or completed orders")
	}

	orderItems := make([]*types.DBOrderItem, 0, len(req.Items))
	for _, itemReq := range req.Items {
		menuItemID := itemReq.MenuItemID
		if menuItemID == "" {
			menuItemID = itemReq.MenuItemId
		}
		if menuItemID == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "menu item ID is required")
		}
		if itemReq.Quantity <= 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "item quantity must be greater than zero")
		}

		menuItem, err := h.repo.GetMenuItemByID(c.Request().Context(), menuItemID)
		if err != nil {
			logger.ErrorWithErr("Failed to get menu item for order update", err, "menu_item_id", menuItemID)
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get menu item")
		}
		if menuItem == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "menu item not found: "+menuItemID)
		}

		// Re-capture menu metadata at edit time.
		orderItems = append(orderItems, &types.DBOrderItem{
			MenuItemID: menuItemID,
			ItemName:   menuItem.Name,
			UnitPrice:  menuItem.Price,
			Quantity:   itemReq.Quantity,
		})
	}

	order.VehicleDescription = strings.TrimSpace(vehicleDesc)
	if err := h.repo.UpdateOrderWithItems(c.Request().Context(), order, orderItems); err != nil {
		logger.ErrorWithErr("Failed to update order with items", err, "order_id", orderID)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update order")
	}

	h.BroadcastOrderUpdate(c.Request().Context(), order)
	h.BroadcastStatsUpdate(c.Request().Context())

	return h.getOrderResponse(c, orderID)
}

// UpdateOrderStatus handles PUT /api/orders/:id/status
func (h *OrderHandler) UpdateOrderStatus(c echo.Context) error {
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid order ID")
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Normalize and validate status
	normalizedStatus := denormalizeStatus(req.Status)
	if normalizedStatus == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid status")
	}

	// Get existing order
	order, err := h.repo.GetOrderByID(c.Request().Context(), orderID)
	if err != nil {
		logger.ErrorWithErr("Failed to get order for status update", err, "order_id", orderID)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get order")
	}
	if order == nil {
		logger.Warn("Order not found for status update", "order_id", orderID)
		return echo.NewHTTPError(http.StatusNotFound, "order not found")
	}

	// Update status
	oldStatus := order.Status
	order.Status = normalizedStatus
	if err := h.repo.UpdateOrder(c.Request().Context(), order); err != nil {
		logger.ErrorWithErr("Failed to update order status", err,
			"order_id", orderID,
			"old_status", oldStatus,
			"new_status", normalizedStatus,
		)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update order")
	}

	logger.Info("Order status updated", "order_id", orderID, "old_status", oldStatus, "new_status", normalizedStatus)

	// Broadcast update to WebSocket clients
	h.BroadcastOrderUpdate(c.Request().Context(), order)

	// Broadcast stats update
	h.BroadcastStatsUpdate(c.Request().Context())

	return h.getOrderResponse(c, orderID)
}

// PurgeOrdersRequest represents the request body for purging orders
type PurgeOrdersRequest struct {
	Scope string `json:"scope"` // "today" or "all"
}

// PurgeOrders handles DELETE /api/orders/purge
func (h *OrderHandler) PurgeOrders(c echo.Context) error {
	var req PurgeOrdersRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	var count int
	var err error

	switch req.Scope {
	case "today":
		count, err = h.repo.PurgeOrdersToday(c.Request().Context())
	case "all":
		count, err = h.repo.PurgeAllOrders(c.Request().Context())
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "scope must be 'today' or 'all'")
	}

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to purge orders")
	}

	// Broadcast stats update after purging
	h.BroadcastStatsUpdate(c.Request().Context())

	return c.JSON(http.StatusOK, map[string]interface{}{
		"deleted": count,
		"scope":   req.Scope,
	})
}

// normalizeStatus converts database status to API status format
func normalizeStatus(status string) string {
	switch status {
	case "NEW":
		return "new"
	case "IN_PROGRESS":
		return "in-progress"
	case "COMPLETED":
		return "completed"
	default:
		return strings.ToLower(status)
	}
}

// denormalizeStatus converts API status format to database status
func denormalizeStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "new":
		return "NEW"
	case "in-progress":
		return "IN_PROGRESS"
	case "completed":
		return "COMPLETED"
	default:
		return ""
	}
}

// HandleWebSocket handles WebSocket connections for real-time order updates
func (h *OrderHandler) HandleWebSocket(c echo.Context) error {
	ws, err := h.upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	// Register client
	h.clientsLock.Lock()
	h.clients[ws] = true
	h.clientsLock.Unlock()

	// Unregister on disconnect
	defer func() {
		h.clientsLock.Lock()
		delete(h.clients, ws)
		h.clientsLock.Unlock()
	}()

	ctx := c.Request().Context()

	// Send current session info
	session, _ := h.repo.GetActiveSession(ctx)
	if session != nil {
		orderCount, revenue, _ := h.repo.GetSessionStats(ctx, session.ID)
		var closedAt *string
		if session.ClosedAt != nil {
			ca := session.ClosedAt.Format(time.RFC3339)
			closedAt = &ca
		}
		sessionMsg := map[string]interface{}{
			"type": "session_update",
			"session": map[string]interface{}{
				"id":                session.ID,
				"eventName":         session.EventName,
				"startedAt":         session.StartedAt.Format(time.RFC3339),
				"expiresAt":         session.ExpiresAt.Format(time.RFC3339),
				"closedAt":          closedAt,
				"status":            session.Status,
				"currentOrderCount": orderCount,
				"currentRevenue":    revenue,
			},
		}
		ws.WriteJSON(sessionMsg)
	} else {
		ws.WriteJSON(map[string]interface{}{
			"type":   "session_update",
			"active": false,
		})
	}

	// Send initial orders (uses captured prices now)
	orders, err := h.repo.GetOrders(ctx)
	if err == nil {
		for _, order := range orders {
			items, _ := h.repo.GetOrderItems(ctx, order.ID)
			itemResponses := make([]OrderItemResponse, 0, len(items))
			var total float64
			for _, item := range items {
				total += item.UnitPrice * float64(item.Quantity)
				itemResponses = append(itemResponses, OrderItemResponse{
					ID:           item.ID,
					MenuItemID:   item.MenuItemID,
					MenuItemId:   item.MenuItemID,
					MenuItemName: item.ItemName,
					Price:        item.UnitPrice,
					Quantity:     item.Quantity,
				})
			}

			msg := map[string]interface{}{
				"type": "order_update",
				"order": OrderResponse{
					ID:                 order.ID,
					DailyOrderNumber:   order.DailyOrderNumber,
					VehicleDescription: order.VehicleDescription,
					CustomerName:       order.VehicleDescription,
					Status:             normalizeStatus(order.Status),
					Items:              itemResponses,
					Total:              total,
					CreatedAt:          order.CreatedAt.Format(time.RFC3339),
					UpdatedAt:          order.UpdatedAt.Format(time.RFC3339),
				},
			}
			if err := ws.WriteJSON(msg); err != nil {
				return err
			}
		}
	}

	// Send initial stats to this client
	h.sendStatsToClient(ctx, ws, orders)

	// Keep connection alive and handle incoming messages
	for {
		var msg map[string]interface{}
		if err := ws.ReadJSON(&msg); err != nil {
			break
		}
		// Echo back or handle client messages if needed
	}

	return nil
}

// BroadcastNewOrder sends a new order notification to all connected WebSocket clients
func (h *OrderHandler) BroadcastNewOrder(ctx context.Context, order *types.DBOrder) {
	// Get items for this order (now includes captured name and price)
	orderItems, _ := h.repo.GetOrderItems(ctx, order.ID)
	itemResponses := make([]OrderItemResponse, 0, len(orderItems))
	var total float64
	for _, item := range orderItems {
		total += item.UnitPrice * float64(item.Quantity)
		itemResponses = append(itemResponses, OrderItemResponse{
			ID:           item.ID,
			MenuItemID:   item.MenuItemID,
			MenuItemId:   item.MenuItemID,
			MenuItemName: item.ItemName,
			Price:        item.UnitPrice,
			Quantity:     item.Quantity,
		})
	}

	msg := map[string]interface{}{
		"type": "order_new",
		"order": OrderResponse{
			ID:                 order.ID,
			DailyOrderNumber:   order.DailyOrderNumber,
			VehicleDescription: order.VehicleDescription,
			CustomerName:       order.VehicleDescription,
			Status:             normalizeStatus(order.Status),
			Items:              itemResponses,
			Total:              total,
			CreatedAt:          order.CreatedAt.Format(time.RFC3339),
			UpdatedAt:          order.UpdatedAt.Format(time.RFC3339),
		},
	}

	data, _ := json.Marshal(msg)
	h.broadcastToClients(data)
}

// BroadcastOrderUpdate sends order updates to all connected WebSocket clients
func (h *OrderHandler) BroadcastOrderUpdate(ctx context.Context, order *types.DBOrder) {
	// Get items for this order (now includes captured name and price)
	orderItems, _ := h.repo.GetOrderItems(ctx, order.ID)
	itemResponses := make([]OrderItemResponse, 0, len(orderItems))
	var total float64
	for _, item := range orderItems {
		total += item.UnitPrice * float64(item.Quantity)
		itemResponses = append(itemResponses, OrderItemResponse{
			ID:           item.ID,
			MenuItemID:   item.MenuItemID,
			MenuItemId:   item.MenuItemID,
			MenuItemName: item.ItemName,
			Price:        item.UnitPrice,
			Quantity:     item.Quantity,
		})
	}

	msg := map[string]interface{}{
		"type": "order_update",
		"order": OrderResponse{
			ID:                 order.ID,
			DailyOrderNumber:   order.DailyOrderNumber,
			VehicleDescription: order.VehicleDescription,
			CustomerName:       order.VehicleDescription,
			Status:             normalizeStatus(order.Status),
			Items:              itemResponses,
			Total:              total,
			CreatedAt:          order.CreatedAt.Format(time.RFC3339),
			UpdatedAt:          order.UpdatedAt.Format(time.RFC3339),
		},
	}

	data, _ := json.Marshal(msg)
	h.broadcastToClients(data)
}

// BroadcastSessionUpdate sends session updates to all connected WebSocket clients
func (h *OrderHandler) BroadcastSessionUpdate(ctx context.Context, session *types.DBSession) {
	orderCount, revenue, _ := h.repo.GetSessionStats(ctx, session.ID)
	var closedAt *string
	if session.ClosedAt != nil {
		ca := session.ClosedAt.Format(time.RFC3339)
		closedAt = &ca
	}

	msg := map[string]interface{}{
		"type":   "session_update",
		"active": true,
		"session": map[string]interface{}{
			"id":                session.ID,
			"eventName":         session.EventName,
			"startedAt":         session.StartedAt.Format(time.RFC3339),
			"expiresAt":         session.ExpiresAt.Format(time.RFC3339),
			"closedAt":          closedAt,
			"status":            session.Status,
			"currentOrderCount": orderCount,
			"currentRevenue":    revenue,
		},
	}

	data, _ := json.Marshal(msg)
	h.broadcastToClients(data)
}

// BroadcastSessionClosed sends session closed notification to all connected WebSocket clients
func (h *OrderHandler) BroadcastSessionClosed(ctx context.Context, session *types.DBSession) {
	var closedAt *string
	if session.ClosedAt != nil {
		ca := session.ClosedAt.Format(time.RFC3339)
		closedAt = &ca
	}

	msg := map[string]interface{}{
		"type":   "session_closed",
		"active": false,
		"session": map[string]interface{}{
			"id":              session.ID,
			"eventName":       session.EventName,
			"startedAt":       session.StartedAt.Format(time.RFC3339),
			"expiresAt":       session.ExpiresAt.Format(time.RFC3339),
			"closedAt":        closedAt,
			"status":          session.Status,
			"finalOrderCount": session.FinalOrderCount,
			"finalRevenue":    session.FinalRevenue,
		},
	}

	data, _ := json.Marshal(msg)
	h.broadcastToClients(data)
}

// StatsResponse represents statistics in the API response
type StatsResponse struct {
	TotalOrders int     `json:"totalOrders"`
	OrdersToday int     `json:"ordersToday"`
	Revenue     float64 `json:"revenue"`
}

// sendStatsToClient sends current stats to a single WebSocket client
func (h *OrderHandler) sendStatsToClient(ctx context.Context, ws *websocket.Conn, orders []types.DBOrder) {
	// Calculate total revenue from captured prices in order items
	totalRevenue := 0.0
	for _, order := range orders {
		orderItems, err := h.repo.GetOrderItems(ctx, order.ID)
		if err != nil {
			continue
		}
		for _, orderItem := range orderItems {
			totalRevenue += orderItem.UnitPrice * float64(orderItem.Quantity)
		}
	}

	// For session-based stats, ordersToday = total orders in current session
	msg := map[string]interface{}{
		"type": "stats_update",
		"stats": StatsResponse{
			TotalOrders: len(orders),
			OrdersToday: len(orders), // In session context, these are today's orders
			Revenue:     totalRevenue,
		},
	}
	ws.WriteJSON(msg)
}

// BroadcastStatsUpdate calculates and broadcasts statistics updates to all connected WebSocket clients
func (h *OrderHandler) BroadcastStatsUpdate(ctx context.Context) {
	// Load orders for current active session
	orders, err := h.repo.GetOrders(ctx)
	if err != nil {
		logger.ErrorWithErr("Failed to load orders for stats", err)
		return
	}

	// Calculate total revenue from captured prices in order items
	totalRevenue := 0.0
	for _, order := range orders {
		orderItems, err := h.repo.GetOrderItems(ctx, order.ID)
		if err != nil {
			continue
		}
		for _, orderItem := range orderItems {
			totalRevenue += orderItem.UnitPrice * float64(orderItem.Quantity)
		}
	}

	// Create stats message (in session context, ordersToday = session orders)
	msg := map[string]interface{}{
		"type": "stats_update",
		"stats": StatsResponse{
			TotalOrders: len(orders),
			OrdersToday: len(orders),
			Revenue:     totalRevenue,
		},
	}

	data, _ := json.Marshal(msg)
	h.broadcastToClients(data)
}

// broadcastToClients sends data to all connected WebSocket clients without holding locks during I/O.
// Dead clients are collected and removed after all sends complete.
func (h *OrderHandler) broadcastToClients(data []byte) {
	// Copy client list under read lock
	h.clientsLock.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.clientsLock.RUnlock()

	// Send to all clients without holding any lock
	var deadClients []*websocket.Conn
	for _, client := range clients {
		if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
			deadClients = append(deadClients, client)
		}
	}

	// Remove dead clients under write lock (only if there are any)
	if len(deadClients) > 0 {
		h.clientsLock.Lock()
		for _, client := range deadClients {
			delete(h.clients, client)
		}
		h.clientsLock.Unlock()
	}
}
