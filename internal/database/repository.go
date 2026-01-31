package database

import (
	"context"
	"time"

	"github.com/jbohanon/fish-fry-orders-v2/internal/types"
)

// Repository defines the interface for database operations
type Repository interface {
	// Menu items
	GetMenuItems(ctx context.Context) ([]types.DBMenuItem, error)
	GetMenuItemByID(ctx context.Context, id string) (*types.DBMenuItem, error)
	CreateMenuItem(ctx context.Context, item *types.DBMenuItem) error
	UpdateMenuItem(ctx context.Context, item *types.DBMenuItem) error
	DeleteMenuItem(ctx context.Context, id string) error
	UpdateMenuItemsOrder(ctx context.Context, itemOrders map[string]int) error

	// Orders
	GetOrders(ctx context.Context) ([]types.DBOrder, error)
	GetOrderByID(ctx context.Context, id int) (*types.DBOrder, error)
	GetNextOrderID(ctx context.Context) (int, error)
	CreateOrder(ctx context.Context, order *types.DBOrder) error
	UpdateOrder(ctx context.Context, order *types.DBOrder) error
	DeleteOrder(ctx context.Context, id int) error
	PurgeOrdersToday(ctx context.Context) (int, error)
	PurgeAllOrders(ctx context.Context) (int, error)

	// Order items
	GetOrderItems(ctx context.Context, orderID int) ([]types.DBOrderItem, error)
	CreateOrderItem(ctx context.Context, item *types.DBOrderItem) error
	UpdateOrderItem(ctx context.Context, item *types.DBOrderItem) error
	DeleteOrderItem(ctx context.Context, id string) error

	// Chat messages
	GetChatMessages(ctx context.Context, orderID int) ([]types.DBChatMessage, error)
	CreateChatMessage(ctx context.Context, message *types.DBChatMessage) error

	// Statistics
	GetOrderStatistics(ctx context.Context, startTime, endTime time.Time) (*types.DBOrderStatistics, error)
}

// NewRepository creates a new repository instance
func NewRepository(dsn string) (Repository, error) {
	// TODO: Implement PostgreSQL repository
	return nil, nil
}
