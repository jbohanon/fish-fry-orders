package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jbohanon/fish-fry-orders-v2/internal/database"
	"github.com/jbohanon/fish-fry-orders-v2/internal/logger"
	"github.com/jbohanon/fish-fry-orders-v2/internal/types"
	"github.com/labstack/echo/v4"
)

type SessionHandler struct {
	repo         database.Repository
	orderHandler *OrderHandler // For broadcasting updates
}

func NewSessionHandler(repo database.Repository, orderHandler *OrderHandler) *SessionHandler {
	return &SessionHandler{
		repo:         repo,
		orderHandler: orderHandler,
	}
}

// SessionResponse represents a session in the API response
type SessionResponse struct {
	ID              int        `json:"id"`
	EventName       string     `json:"eventName"`
	StartedAt       string     `json:"startedAt"`
	ExpiresAt       string     `json:"expiresAt"`
	ClosedAt        *string    `json:"closedAt,omitempty"`
	Status          string     `json:"status"`
	FinalOrderCount *int       `json:"finalOrderCount,omitempty"`
	FinalRevenue    *float64   `json:"finalRevenue,omitempty"`
	Notes           string     `json:"notes,omitempty"`
	// Live stats (for active sessions)
	CurrentOrderCount int     `json:"currentOrderCount,omitempty"`
	CurrentRevenue    float64 `json:"currentRevenue,omitempty"`
}

// CreateSessionRequest represents the request to create a session
type CreateSessionRequest struct {
	EventName string `json:"eventName"`
	ExpiresAt string `json:"expiresAt,omitempty"` // ISO 8601 format
	Notes     string `json:"notes,omitempty"`
}

// UpdateSessionRequest represents the request to update a session
type UpdateSessionRequest struct {
	EventName string `json:"eventName,omitempty"`
	ExpiresAt string `json:"expiresAt,omitempty"` // ISO 8601 format
	Notes     string `json:"notes,omitempty"`
}

// SessionComparisonResponse represents the comparison stats response
type SessionComparisonResponse struct {
	Sessions      []SessionComparisonItem `json:"sessions"`
	TotalOrders   int                     `json:"totalOrders"`
	TotalRevenue  float64                 `json:"totalRevenue"`
	ItemBreakdown []ItemBreakdownResponse `json:"itemBreakdown"`
}

type SessionComparisonItem struct {
	SessionID  int     `json:"sessionId"`
	EventName  string  `json:"eventName"`
	StartedAt  string  `json:"startedAt"`
	OrderCount int     `json:"orderCount"`
	Revenue    float64 `json:"revenue"`
}

type ItemBreakdownResponse struct {
	ItemName string  `json:"itemName"`
	Quantity int     `json:"quantity"`
	Revenue  float64 `json:"revenue"`
	Percent  float64 `json:"percent"`
}

func sessionToResponse(session *types.DBSession, orderCount int, revenue float64) SessionResponse {
	resp := SessionResponse{
		ID:                session.ID,
		EventName:         session.EventName,
		StartedAt:         session.StartedAt.Format(time.RFC3339),
		ExpiresAt:         session.ExpiresAt.Format(time.RFC3339),
		Status:            session.Status,
		Notes:             session.Notes,
		FinalOrderCount:   session.FinalOrderCount,
		FinalRevenue:      session.FinalRevenue,
		CurrentOrderCount: orderCount,
		CurrentRevenue:    revenue,
	}
	if session.ClosedAt != nil {
		closedAt := session.ClosedAt.Format(time.RFC3339)
		resp.ClosedAt = &closedAt
	}
	return resp
}

// GetCurrentSession handles GET /api/session
func (h *SessionHandler) GetCurrentSession(c echo.Context) error {
	session, err := h.repo.GetActiveSession(c.Request().Context())
	if err != nil {
		logger.ErrorWithErr("Failed to get active session", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get active session")
	}

	if session == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"active": false,
		})
	}

	// Get current stats for active session
	orderCount, revenue, err := h.repo.GetSessionStats(c.Request().Context(), session.ID)
	if err != nil {
		logger.ErrorWithErr("Failed to get session stats", err, "session_id", session.ID)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"active":  true,
		"session": sessionToResponse(session, orderCount, revenue),
	})
}

// CreateSession handles POST /api/session
func (h *SessionHandler) CreateSession(c echo.Context) error {
	var req CreateSessionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Check if there's already an active session
	existing, err := h.repo.GetActiveSession(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check for active session")
	}
	if existing != nil {
		return echo.NewHTTPError(http.StatusConflict, "an active session already exists")
	}

	now := time.Now()
	session := &types.DBSession{
		EventName: req.EventName,
		StartedAt: now,
		Status:    "ACTIVE",
		Notes:     req.Notes,
	}

	// Default event name if not provided
	if session.EventName == "" {
		session.EventName = "Fish Fry " + now.Format("2006-01-02")
	}

	// Parse expiry time or default to end of day
	if req.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid expiresAt format")
		}
		session.ExpiresAt = expiresAt
	} else {
		// Default to end of today
		session.ExpiresAt = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	}

	if err := h.repo.CreateSession(c.Request().Context(), session); err != nil {
		logger.ErrorWithErr("Failed to create session", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create session")
	}

	logger.Info("Session created", "session_id", session.ID, "event_name", session.EventName)

	// Broadcast session update
	h.orderHandler.BroadcastSessionUpdate(c.Request().Context(), session)

	return c.JSON(http.StatusCreated, sessionToResponse(session, 0, 0))
}

// GetSession handles GET /api/sessions/:id
func (h *SessionHandler) GetSession(c echo.Context) error {
	sessionID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid session ID")
	}

	session, err := h.repo.GetSessionByID(c.Request().Context(), sessionID)
	if err != nil {
		logger.ErrorWithErr("Failed to get session", err, "session_id", sessionID)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get session")
	}
	if session == nil {
		return echo.NewHTTPError(http.StatusNotFound, "session not found")
	}

	orderCount, revenue, _ := h.repo.GetSessionStats(c.Request().Context(), sessionID)
	return c.JSON(http.StatusOK, sessionToResponse(session, orderCount, revenue))
}

// UpdateSession handles PUT /api/session/:id
func (h *SessionHandler) UpdateSession(c echo.Context) error {
	sessionID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid session ID")
	}

	var req UpdateSessionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	session, err := h.repo.GetSessionByID(c.Request().Context(), sessionID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get session")
	}
	if session == nil {
		return echo.NewHTTPError(http.StatusNotFound, "session not found")
	}

	// Can only update active sessions
	if session.Status != "ACTIVE" {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot modify a closed session")
	}

	// Update fields
	if req.EventName != "" {
		session.EventName = req.EventName
	}
	if req.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid expiresAt format")
		}
		session.ExpiresAt = expiresAt
	}
	if req.Notes != "" {
		session.Notes = req.Notes
	}

	if err := h.repo.UpdateSession(c.Request().Context(), session); err != nil {
		logger.ErrorWithErr("Failed to update session", err, "session_id", sessionID)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update session")
	}

	logger.Info("Session updated", "session_id", sessionID, "event_name", session.EventName, "expires_at", session.ExpiresAt)

	// Broadcast session update
	h.orderHandler.BroadcastSessionUpdate(c.Request().Context(), session)

	orderCount, revenue, _ := h.repo.GetSessionStats(c.Request().Context(), sessionID)
	return c.JSON(http.StatusOK, sessionToResponse(session, orderCount, revenue))
}

// CloseSession handles POST /api/session/:id/close
func (h *SessionHandler) CloseSession(c echo.Context) error {
	sessionID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid session ID")
	}

	session, err := h.repo.GetSessionByID(c.Request().Context(), sessionID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get session")
	}
	if session == nil {
		return echo.NewHTTPError(http.StatusNotFound, "session not found")
	}

	if session.Status == "CLOSED" {
		return echo.NewHTTPError(http.StatusBadRequest, "session is already closed")
	}

	if err := h.repo.CloseSession(c.Request().Context(), sessionID); err != nil {
		logger.ErrorWithErr("Failed to close session", err, "session_id", sessionID)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to close session")
	}

	// Fetch updated session
	session, _ = h.repo.GetSessionByID(c.Request().Context(), sessionID)

	logger.Info("Session closed", "session_id", sessionID, "final_order_count", session.FinalOrderCount, "final_revenue", session.FinalRevenue)

	// Broadcast session closed
	h.orderHandler.BroadcastSessionClosed(c.Request().Context(), session)

	orderCount := 0
	revenue := 0.0
	if session.FinalOrderCount != nil {
		orderCount = *session.FinalOrderCount
	}
	if session.FinalRevenue != nil {
		revenue = *session.FinalRevenue
	}
	return c.JSON(http.StatusOK, sessionToResponse(session, orderCount, revenue))
}

// GetSessions handles GET /api/sessions
func (h *SessionHandler) GetSessions(c echo.Context) error {
	var from, to *time.Time

	// Parse optional date filters
	if fromStr := c.QueryParam("from"); fromStr != "" {
		t, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid 'from' date format")
		}
		from = &t
	}
	if toStr := c.QueryParam("to"); toStr != "" {
		t, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid 'to' date format")
		}
		// Set to end of day
		endOfDay := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
		to = &endOfDay
	}

	sessions, err := h.repo.GetSessions(c.Request().Context(), from, to)
	if err != nil {
		logger.ErrorWithErr("Failed to get sessions", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get sessions")
	}

	responses := make([]SessionResponse, 0, len(sessions))
	for _, session := range sessions {
		orderCount, revenue, _ := h.repo.GetSessionStats(c.Request().Context(), session.ID)
		responses = append(responses, sessionToResponse(&session, orderCount, revenue))
	}

	return c.JSON(http.StatusOK, responses)
}

// GetSessionOrders handles GET /api/sessions/:id/orders
func (h *SessionHandler) GetSessionOrders(c echo.Context) error {
	sessionID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid session ID")
	}

	session, err := h.repo.GetSessionByID(c.Request().Context(), sessionID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get session")
	}
	if session == nil {
		return echo.NewHTTPError(http.StatusNotFound, "session not found")
	}

	orders, err := h.repo.GetOrdersBySession(c.Request().Context(), sessionID)
	if err != nil {
		logger.ErrorWithErr("Failed to get session orders", err, "session_id", sessionID)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get session orders")
	}

	responses := make([]OrderResponse, 0, len(orders))
	for _, order := range orders {
		items, err := h.repo.GetOrderItems(c.Request().Context(), order.ID)
		if err != nil {
			continue
		}

		var total float64
		itemResponses := make([]OrderItemResponse, 0, len(items))
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

		responses = append(responses, OrderResponse{
			ID:                 order.ID,
			DailyOrderNumber:   order.DailyOrderNumber,
			VehicleDescription: order.VehicleDescription,
			CustomerName:       order.VehicleDescription,
			Status:             normalizeStatus(order.Status),
			Items:              itemResponses,
			Total:              total,
			CreatedAt:          order.CreatedAt.Format(time.RFC3339),
			UpdatedAt:          order.UpdatedAt.Format(time.RFC3339),
		})
	}

	return c.JSON(http.StatusOK, responses)
}

// CompareSessions handles GET /api/sessions/compare?ids=1,2,3
func (h *SessionHandler) CompareSessions(c echo.Context) error {
	idsParam := c.QueryParam("ids")
	if idsParam == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "ids parameter is required")
	}

	// Parse comma-separated IDs
	var sessionIDs []int
	for _, idStr := range splitAndTrim(idsParam) {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		sessionIDs = append(sessionIDs, id)
	}

	if len(sessionIDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "no valid session IDs provided")
	}

	stats, err := h.repo.CompareSessionStats(c.Request().Context(), sessionIDs)
	if err != nil {
		logger.ErrorWithErr("Failed to compare sessions", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to compare sessions")
	}

	// Aggregate stats
	var totalOrders int
	var totalRevenue float64
	itemTotals := make(map[string]database.ItemStats)

	sessions := make([]SessionComparisonItem, 0, len(stats))
	for _, s := range stats {
		totalOrders += s.OrderCount
		totalRevenue += s.Revenue

		sessions = append(sessions, SessionComparisonItem{
			SessionID:  s.SessionID,
			EventName:  s.EventName,
			StartedAt:  s.StartedAt.Format(time.RFC3339),
			OrderCount: s.OrderCount,
			Revenue:    s.Revenue,
		})

		for name, item := range s.ItemBreakdown {
			existing := itemTotals[name]
			existing.ItemName = item.ItemName
			existing.Quantity += item.Quantity
			existing.Revenue += item.Revenue
			itemTotals[name] = existing
		}
	}

	// Convert to response
	breakdown := make([]ItemBreakdownResponse, 0, len(itemTotals))
	for _, item := range itemTotals {
		percent := 0.0
		if totalRevenue > 0 {
			percent = (item.Revenue / totalRevenue) * 100
		}
		breakdown = append(breakdown, ItemBreakdownResponse{
			ItemName: item.ItemName,
			Quantity: item.Quantity,
			Revenue:  item.Revenue,
			Percent:  percent,
		})
	}

	return c.JSON(http.StatusOK, SessionComparisonResponse{
		Sessions:      sessions,
		TotalOrders:   totalOrders,
		TotalRevenue:  totalRevenue,
		ItemBreakdown: breakdown,
	})
}

// Helper function to split and trim comma-separated values
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
