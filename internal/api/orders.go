package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jbohanon/fish-fry-orders-v2/internal/database"
	"github.com/jbohanon/fish-fry-orders-v2/internal/logger"
	"github.com/jbohanon/fish-fry-orders-v2/internal/types"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

type OrderHandler struct {
	repo        database.Repository
	clients     map[*websocket.Conn]bool
	clientsLock sync.RWMutex
}

func NewOrderHandler(repo database.Repository) *OrderHandler {
	return &OrderHandler{
		repo:    repo,
		clients: make(map[*websocket.Conn]bool),
	}
}

// CreateOrderRequest represents the request to create an order
type CreateOrderRequest struct {
	VehicleDescription string                 `json:"vehicle_description"`
	CustomerName       string                 `json:"customerName"` // Frontend uses this
	Items              []CreateOrderItemRequest `json:"items"`
}

type CreateOrderItemRequest struct {
	MenuItemID string `json:"menu_item_id"`
	MenuItemId string `json:"menuItemId"` // Frontend uses camelCase
	Quantity   int32  `json:"quantity"`
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

	// Create order
	order := &types.DBOrder{
		VehicleDescription: vehicleDesc,
		Status:             "NEW",
	}

	if err := h.repo.CreateOrder(c.Request().Context(), order); err != nil {
		logger.ErrorWithErr("Failed to create order", err,
			"vehicle_description", vehicleDesc,
			"items_count", len(req.Items),
		)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create order")
	}

	logger.Info("Order created successfully", "order_id", order.ID, "vehicle_description", vehicleDesc)

	// Create order items
	for i, itemReq := range req.Items {
		menuItemID := itemReq.MenuItemID
		if menuItemID == "" {
			menuItemID = itemReq.MenuItemId // Use camelCase if snake_case is empty
		}
		orderItem := &types.DBOrderItem{
			OrderID:    order.ID,
			MenuItemID: menuItemID,
			Quantity:   itemReq.Quantity,
		}
		if err := h.repo.CreateOrderItem(c.Request().Context(), orderItem); err != nil {
			logger.ErrorWithErr("Failed to create order item", err,
				"order_id", order.ID,
				"menu_item_id", menuItemID,
				"quantity", itemReq.Quantity,
				"item_index", i,
			)
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to create order item")
		}
	}

	// Fetch the complete order with items for response
	createdOrder, err := h.repo.GetOrderByID(c.Request().Context(), order.ID)
	if err != nil || createdOrder == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get created order")
	}
	
	// Broadcast new order to WebSocket clients
	h.BroadcastNewOrder(c.Request().Context(), createdOrder)
	
	// Broadcast stats update
	h.BroadcastStatsUpdate(c.Request().Context())
	
	return h.getOrderResponse(c, order.ID)
}

// GetOrders handles GET /api/orders
func (h *OrderHandler) GetOrders(c echo.Context) error {
	orders, err := h.repo.GetOrders(c.Request().Context())
	if err != nil {
		logger.ErrorWithErr("Failed to get orders", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get orders")
	}

	// Get menu items for name/price lookup
	menuItems, err := h.repo.GetMenuItems(c.Request().Context())
	if err != nil {
		logger.ErrorWithErr("Failed to get menu items", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get menu items")
	}
	menuItemMap := make(map[string]types.DBMenuItem)
	for _, mi := range menuItems {
		menuItemMap[mi.ID] = mi
	}

	responses := make([]OrderResponse, 0, len(orders))
	for _, order := range orders {
		// Get order items
		items, err := h.repo.GetOrderItems(c.Request().Context(), order.ID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get order items")
		}

		var total float64
		itemResponses := make([]OrderItemResponse, 0, len(items))
		for _, item := range items {
			var menuItemName string
			var price float64
			if mi, ok := menuItemMap[item.MenuItemID]; ok {
				menuItemName = mi.Name
				price = mi.Price
			}
			total += price * float64(item.Quantity)
			itemResponses = append(itemResponses, OrderItemResponse{
				ID:           item.ID,
				MenuItemID:   item.MenuItemID,
				MenuItemName: menuItemName,
				Price:        price,
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

	// Get order items
	items, err := h.repo.GetOrderItems(c.Request().Context(), orderID)
	if err != nil {
		logger.ErrorWithErr("Failed to get order items", err, "order_id", orderID)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get order items")
	}

	// Get menu items for name/price lookup
	menuItems, err := h.repo.GetMenuItems(c.Request().Context())
	if err != nil {
		logger.ErrorWithErr("Failed to get menu items", err, "order_id", orderID)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get menu items")
	}
	menuItemMap := make(map[string]types.DBMenuItem)
	for _, mi := range menuItems {
		menuItemMap[mi.ID] = mi
	}

	var total float64
	itemResponses := make([]OrderItemResponse, 0, len(items))
	for _, item := range items {
		var menuItemName string
		var price float64
		if mi, ok := menuItemMap[item.MenuItemID]; ok {
			menuItemName = mi.Name
			price = mi.Price
		}
		total += price * float64(item.Quantity)
		itemResponses = append(itemResponses, OrderItemResponse{
			ID:           item.ID,
			MenuItemID:   item.MenuItemID,
			MenuItemId:   item.MenuItemID, // Also set camelCase version
			MenuItemName: menuItemName,
			Price:        price,
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
	switch status {
	case "new":
		return "NEW"
	case "in-progress":
		return "IN_PROGRESS"
	case "completed":
		return "COMPLETED"
	default:
		return strings.ToUpper(status)
	}
}

// HandleWebSocket handles WebSocket connections for real-time order updates
func (h *OrderHandler) HandleWebSocket(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
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

	// Get menu items for name/price lookup
	menuItems, _ := h.repo.GetMenuItems(c.Request().Context())
	menuItemMap := make(map[string]types.DBMenuItem)
	for _, mi := range menuItems {
		menuItemMap[mi.ID] = mi
	}

	// Send initial orders
	orders, err := h.repo.GetOrders(c.Request().Context())
	if err == nil {
		for _, order := range orders {
			items, _ := h.repo.GetOrderItems(c.Request().Context(), order.ID)
			itemResponses := make([]OrderItemResponse, 0, len(items))
			for _, item := range items {
				var menuItemName string
				var price float64
				if mi, ok := menuItemMap[item.MenuItemID]; ok {
					menuItemName = mi.Name
					price = mi.Price
				}
				itemResponses = append(itemResponses, OrderItemResponse{
					ID:           item.ID,
					MenuItemID:   item.MenuItemID,
					MenuItemId:   item.MenuItemID,
					MenuItemName: menuItemName,
					Price:        price,
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
					CreatedAt:          order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
					UpdatedAt:          order.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
				},
			}
			if err := ws.WriteJSON(msg); err != nil {
				return err
			}
		}
	}

	// Send initial stats to this client
	h.sendStatsToClient(c.Request().Context(), ws, menuItems, orders)

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
	// Get menu items for name/price lookup
	menuItems, _ := h.repo.GetMenuItems(ctx)
	menuItemMap := make(map[string]types.DBMenuItem)
	for _, mi := range menuItems {
		menuItemMap[mi.ID] = mi
	}

	// Get items for this order
	orderItems, _ := h.repo.GetOrderItems(ctx, order.ID)
	itemResponses := make([]OrderItemResponse, 0, len(orderItems))
	for _, item := range orderItems {
		var menuItemName string
		var price float64
		if mi, ok := menuItemMap[item.MenuItemID]; ok {
			menuItemName = mi.Name
			price = mi.Price
		}
		itemResponses = append(itemResponses, OrderItemResponse{
			ID:           item.ID,
			MenuItemID:   item.MenuItemID,
			MenuItemId:   item.MenuItemID, // Also set camelCase version
			MenuItemName: menuItemName,
			Price:        price,
			Quantity:     item.Quantity,
		})
	}

	// Fetch the order again to get DailyOrderNumber
	orderWithDailyNum, err := h.repo.GetOrderByID(ctx, order.ID)
	if err != nil || orderWithDailyNum == nil {
		orderWithDailyNum = order // Fallback to original if fetch fails
	}

	msg := map[string]interface{}{
		"type": "order_new",
		"order": OrderResponse{
			ID:                 order.ID,
			DailyOrderNumber:   orderWithDailyNum.DailyOrderNumber,
			VehicleDescription: order.VehicleDescription,
			CustomerName:       order.VehicleDescription, // Alias for frontend
			Status:             normalizeStatus(order.Status),
			Items:              itemResponses,
			CreatedAt:          order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:          order.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	}

	data, _ := json.Marshal(msg)

	h.clientsLock.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.clientsLock.RUnlock()

	// Send to all clients, remove dead ones
	h.clientsLock.Lock()
	for _, client := range clients {
		if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
			delete(h.clients, client)
		}
	}
	h.clientsLock.Unlock()
}

// BroadcastOrderUpdate sends order updates to all connected WebSocket clients
func (h *OrderHandler) BroadcastOrderUpdate(ctx context.Context, order *types.DBOrder) {
	// Get menu items for name/price lookup
	menuItems, _ := h.repo.GetMenuItems(ctx)
	menuItemMap := make(map[string]types.DBMenuItem)
	for _, mi := range menuItems {
		menuItemMap[mi.ID] = mi
	}

	// Get items for this order
	orderItems, _ := h.repo.GetOrderItems(ctx, order.ID)
	itemResponses := make([]OrderItemResponse, 0, len(orderItems))
	for _, item := range orderItems {
		var menuItemName string
		var price float64
		if mi, ok := menuItemMap[item.MenuItemID]; ok {
			menuItemName = mi.Name
			price = mi.Price
		}
		itemResponses = append(itemResponses, OrderItemResponse{
			ID:           item.ID,
			MenuItemID:   item.MenuItemID,
			MenuItemId:   item.MenuItemID, // Also set camelCase version
			MenuItemName: menuItemName,
			Price:        price,
			Quantity:     item.Quantity,
		})
	}

	// Fetch the order again to get DailyOrderNumber
	orderWithDailyNum, err := h.repo.GetOrderByID(ctx, order.ID)
	if err != nil || orderWithDailyNum == nil {
		orderWithDailyNum = order // Fallback to original if fetch fails
	}

	msg := map[string]interface{}{
		"type": "order_update",
		"order": OrderResponse{
			ID:                 order.ID,
			DailyOrderNumber:   orderWithDailyNum.DailyOrderNumber,
			VehicleDescription: order.VehicleDescription,
			CustomerName:       order.VehicleDescription, // Alias for frontend
			Status:             normalizeStatus(order.Status),
			Items:              itemResponses,
			CreatedAt:          order.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:          order.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	}

	data, _ := json.Marshal(msg)

	h.clientsLock.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.clientsLock.RUnlock()

	// Send to all clients, remove dead ones
	h.clientsLock.Lock()
	for _, client := range clients {
		if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
			delete(h.clients, client)
		}
	}
	h.clientsLock.Unlock()
}

// StatsResponse represents statistics in the API response
type StatsResponse struct {
	TotalOrders int     `json:"totalOrders"`
	OrdersToday int     `json:"ordersToday"`
	Revenue     float64 `json:"revenue"`
}

// sendStatsToClient sends current stats to a single WebSocket client
func (h *OrderHandler) sendStatsToClient(ctx context.Context, ws *websocket.Conn, menuItems []types.DBMenuItem, orders []types.DBOrder) {
	// Create a map of menu item IDs to prices for quick lookup
	menuItemPrices := make(map[string]float64)
	for _, item := range menuItems {
		menuItemPrices[item.ID] = item.Price
	}

	// Calculate orders today
	today := time.Now()
	startOfToday := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	ordersToday := 0
	for _, order := range orders {
		if order.CreatedAt.After(startOfToday) || order.CreatedAt.Equal(startOfToday) {
			ordersToday++
		}
	}

	// Calculate total revenue
	totalRevenue := 0.0
	for _, order := range orders {
		// Get order items for this order
		orderItems, err := h.repo.GetOrderItems(ctx, order.ID)
		if err != nil {
			continue // Skip orders with errors loading items
		}
		for _, orderItem := range orderItems {
			if price, ok := menuItemPrices[orderItem.MenuItemID]; ok {
				totalRevenue += price * float64(orderItem.Quantity)
			}
		}
	}

	// Create and send stats message
	msg := map[string]interface{}{
		"type": "stats_update",
		"stats": StatsResponse{
			TotalOrders: len(orders),
			OrdersToday: ordersToday,
			Revenue:     totalRevenue,
		},
	}
	ws.WriteJSON(msg)
}

// BroadcastStatsUpdate calculates and broadcasts statistics updates to all connected WebSocket clients
func (h *OrderHandler) BroadcastStatsUpdate(ctx context.Context) {
	// Load all orders
	orders, err := h.repo.GetOrders(ctx)
	if err != nil {
		logger.ErrorWithErr("Failed to load orders for stats", err)
		return
	}

	// Load all menu items for price lookup
	menuItems, err := h.repo.GetMenuItems(ctx)
	if err != nil {
		logger.ErrorWithErr("Failed to load menu items for stats", err)
		return
	}

	// Create a map of menu item IDs to prices for quick lookup
	menuItemPrices := make(map[string]float64)
	for _, item := range menuItems {
		menuItemPrices[item.ID] = item.Price
	}

	// Calculate orders today
	today := time.Now()
	startOfToday := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	ordersToday := 0
	for _, order := range orders {
		if order.CreatedAt.After(startOfToday) || order.CreatedAt.Equal(startOfToday) {
			ordersToday++
		}
	}

	// Calculate total revenue
	totalRevenue := 0.0
	for _, order := range orders {
		// Get order items for this order
		orderItems, err := h.repo.GetOrderItems(ctx, order.ID)
		if err != nil {
			continue // Skip orders with errors loading items
		}
		for _, orderItem := range orderItems {
			if price, ok := menuItemPrices[orderItem.MenuItemID]; ok {
				totalRevenue += price * float64(orderItem.Quantity)
			}
		}
	}

	// Create stats message
	msg := map[string]interface{}{
		"type": "stats_update",
		"stats": StatsResponse{
			TotalOrders: len(orders),
			OrdersToday: ordersToday,
			Revenue:     totalRevenue,
		},
	}

	data, _ := json.Marshal(msg)

	// Broadcast to all clients
	h.clientsLock.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.clientsLock.RUnlock()

	// Send to all clients, remove dead ones
	h.clientsLock.Lock()
	for _, client := range clients {
		if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
			delete(h.clients, client)
		}
	}
	h.clientsLock.Unlock()
}
