package api

import (
	"net/http"

	"git.nonahob.net/jacob/fish-fry-orders/internal/database"
	"git.nonahob.net/jacob/fish-fry-orders/internal/types"
	"github.com/labstack/echo/v4"
)

type MenuHandler struct {
	repo database.Repository
}

func NewMenuHandler(repo database.Repository) *MenuHandler {
	return &MenuHandler{repo: repo}
}

// MenuItemResponse represents a menu item in the API response
type MenuItemResponse struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	IsActive bool    `json:"is_active"`
}

// GetMenuItems handles GET /api/menu-items
func (h *MenuHandler) GetMenuItems(c echo.Context) error {
	items, err := h.repo.GetMenuItems(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get menu items")
	}

	responses := make([]MenuItemResponse, 0, len(items))
	for _, item := range items {
		// Only return active items (or all if admin - but for now, just active)
		if item.IsActive {
			responses = append(responses, MenuItemResponse{
				ID:       item.ID,
				Name:     item.Name,
				Price:    item.Price,
				IsActive: item.IsActive,
			})
		}
	}

	return c.JSON(http.StatusOK, responses)
}

// GetMenuItem handles GET /api/menu-items/:id
func (h *MenuHandler) GetMenuItem(c echo.Context) error {
	itemID := c.Param("id")

	item, err := h.repo.GetMenuItemByID(c.Request().Context(), itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get menu item")
	}
	if item == nil {
		return echo.NewHTTPError(http.StatusNotFound, "menu item not found")
	}

	return c.JSON(http.StatusOK, MenuItemResponse{
		ID:       item.ID,
		Name:     item.Name,
		Price:    item.Price,
		IsActive: item.IsActive,
	})
}

// CreateMenuItemRequest represents the request body for creating a menu item
type CreateMenuItemRequest struct {
	Name     string  `json:"name" validate:"required"`
	Price    float64 `json:"price" validate:"required,gt=0"`
	IsActive bool    `json:"is_active"`
}

// CreateMenuItem handles POST /api/menu-items
func (h *MenuHandler) CreateMenuItem(c echo.Context) error {
	var req CreateMenuItemRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Validate request
	if req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name is required")
	}
	if req.Price <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "price must be greater than 0")
	}

	// Create menu item (default to active for new items)
	item := &types.DBMenuItem{
		Name:     req.Name,
		Price:    req.Price,
		IsActive: true, // Default new items to active
	}

	if err := h.repo.CreateMenuItem(c.Request().Context(), item); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create menu item")
	}

	return c.JSON(http.StatusCreated, MenuItemResponse{
		ID:       item.ID,
		Name:     item.Name,
		Price:    item.Price,
		IsActive: item.IsActive,
	})
}

// UpdateMenuItemRequest represents the request body for updating a menu item
type UpdateMenuItemRequest struct {
	Name     string  `json:"name" validate:"required"`
	Price    float64 `json:"price" validate:"required,gt=0"`
	IsActive *bool   `json:"is_active"` // Pointer to allow nil (not provided)
}

// UpdateMenuItem handles PUT /api/menu-items/:id
func (h *MenuHandler) UpdateMenuItem(c echo.Context) error {
	itemID := c.Param("id")

	// Check if item exists
	item, err := h.repo.GetMenuItemByID(c.Request().Context(), itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get menu item")
	}
	if item == nil {
		return echo.NewHTTPError(http.StatusNotFound, "menu item not found")
	}

	var req UpdateMenuItemRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Validate price if provided
	if req.Price < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "price must be greater than or equal to 0")
	}

	// Update menu item - only update fields that are provided
	if req.Name != "" {
		item.Name = req.Name
	}
	if req.Price > 0 {
		item.Price = req.Price
	}
	if req.IsActive != nil {
		item.IsActive = *req.IsActive
	}

	if err := h.repo.UpdateMenuItem(c.Request().Context(), item); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update menu item")
	}

	return c.JSON(http.StatusOK, MenuItemResponse{
		ID:       item.ID,
		Name:     item.Name,
		Price:    item.Price,
		IsActive: item.IsActive,
	})
}

// DeleteMenuItem handles DELETE /api/menu-items/:id
func (h *MenuHandler) DeleteMenuItem(c echo.Context) error {
	itemID := c.Param("id")

	// Check if item exists
	item, err := h.repo.GetMenuItemByID(c.Request().Context(), itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get menu item")
	}
	if item == nil {
		return echo.NewHTTPError(http.StatusNotFound, "menu item not found")
	}

	if err := h.repo.DeleteMenuItem(c.Request().Context(), itemID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete menu item")
	}

	return c.NoContent(http.StatusNoContent)
}

// UpdateMenuItemsOrderRequest represents the request body for reordering menu items
type UpdateMenuItemsOrderRequest struct {
	ItemOrders map[string]int `json:"itemOrders"` // map of item ID to display order
}

// UpdateMenuItemsOrder handles PUT /api/menu-items/order
func (h *MenuHandler) UpdateMenuItemsOrder(c echo.Context) error {
	var req UpdateMenuItemsOrderRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if len(req.ItemOrders) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "itemOrders cannot be empty")
	}

	if err := h.repo.UpdateMenuItemsOrder(c.Request().Context(), req.ItemOrders); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update menu items order")
	}

	return c.NoContent(http.StatusNoContent)
}
