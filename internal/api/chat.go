package api

import (
	"net/http"
	"strconv"

	"github.com/jbohanon/fish-fry-orders-v2/internal/database"
	"github.com/jbohanon/fish-fry-orders-v2/internal/types"
	"github.com/labstack/echo/v4"
)

type ChatHandler struct {
	repo database.Repository
}

func NewChatHandler(repo database.Repository) *ChatHandler {
	return &ChatHandler{repo: repo}
}

// CreateMessageRequest represents the request to create a chat message
type CreateMessageRequest struct {
	Content string `json:"content"`
}

// ChatMessageResponse represents a chat message in the API response
type ChatMessageResponse struct {
	ID         string `json:"id"`
	OrderID    int    `json:"order_id"`
	Content    string `json:"content"`
	SenderRole string `json:"sender_role"`
	CreatedAt  string `json:"created_at"`
}

// CreateMessage handles POST /api/orders/:id/messages
func (h *ChatHandler) CreateMessage(c echo.Context) error {
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid order ID")
	}

	// Verify order exists
	order, err := h.repo.GetOrderByID(c.Request().Context(), orderID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get order")
	}
	if order == nil {
		return echo.NewHTTPError(http.StatusNotFound, "order not found")
	}

	var req CreateMessageRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Content == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "content is required")
	}

	// Get user role from auth (simplified - you may want to extract from context)
	// For now, default to WORKER, but this should come from auth middleware
	senderRole := "WORKER" // TODO: Get from auth context

	message := &types.DBChatMessage{
		OrderID:    orderID,
		Content:    req.Content,
		SenderRole: senderRole,
	}

	if err := h.repo.CreateChatMessage(c.Request().Context(), message); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create message")
	}

	return c.JSON(http.StatusOK, ChatMessageResponse{
		ID:         message.ID,
		OrderID:    message.OrderID,
		Content:    message.Content,
		SenderRole: message.SenderRole,
		CreatedAt:  message.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// GetMessages handles GET /api/orders/:id/messages
func (h *ChatHandler) GetMessages(c echo.Context) error {
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid order ID")
	}

	// Verify order exists
	order, err := h.repo.GetOrderByID(c.Request().Context(), orderID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get order")
	}
	if order == nil {
		return echo.NewHTTPError(http.StatusNotFound, "order not found")
	}

	messages, err := h.repo.GetChatMessages(c.Request().Context(), orderID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get messages")
	}

	responses := make([]ChatMessageResponse, 0, len(messages))
	for _, msg := range messages {
		responses = append(responses, ChatMessageResponse{
			ID:         msg.ID,
			OrderID:    msg.OrderID,
			Content:    msg.Content,
			SenderRole: msg.SenderRole,
			CreatedAt:  msg.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return c.JSON(http.StatusOK, responses)
}
