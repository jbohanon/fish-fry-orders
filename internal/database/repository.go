package database

import (
	"context"
	"time"

	"github.com/jbohanon/fish-fry-orders-v2/internal/types"
)

// Repository defines the interface for database operations
type Repository interface {
	// Sessions
	GetActiveSession(ctx context.Context) (*types.DBSession, error)
	GetSessionByID(ctx context.Context, id int) (*types.DBSession, error)
	GetSessions(ctx context.Context, from, to *time.Time) ([]types.DBSession, error)
	CreateSession(ctx context.Context, session *types.DBSession) error
	UpdateSession(ctx context.Context, session *types.DBSession) error
	CloseSession(ctx context.Context, sessionID int) error
	GetOrCreateActiveSession(ctx context.Context) (*types.DBSession, error)
	GetNextDailyOrderNumber(ctx context.Context, sessionID int) (int, error)
	GetSessionStats(ctx context.Context, sessionID int) (orderCount int, revenue float64, err error)
	CompareSessionStats(ctx context.Context, sessionIDs []int) ([]SessionComparisonStats, error)

	// Menu items
	GetMenuItems(ctx context.Context) ([]types.DBMenuItem, error)
	GetMenuItemByID(ctx context.Context, id string) (*types.DBMenuItem, error)
	CreateMenuItem(ctx context.Context, item *types.DBMenuItem) error
	UpdateMenuItem(ctx context.Context, item *types.DBMenuItem) error
	DeleteMenuItem(ctx context.Context, id string) error
	UpdateMenuItemsOrder(ctx context.Context, itemOrders map[string]int) error

	// Orders
	GetOrders(ctx context.Context) ([]types.DBOrder, error)
	GetOrdersBySession(ctx context.Context, sessionID int) ([]types.DBOrder, error)
	GetOrderByID(ctx context.Context, id int) (*types.DBOrder, error)
	GetNextOrderID(ctx context.Context) (int, error)
	CreateOrder(ctx context.Context, order *types.DBOrder) error
	UpdateOrder(ctx context.Context, order *types.DBOrder) error
	DeleteOrder(ctx context.Context, id int) error
	CompleteAllSessionOrders(ctx context.Context, sessionID int) error
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

// SessionComparisonStats holds aggregated stats for session comparison
type SessionComparisonStats struct {
	SessionID   int                `json:"session_id"`
	EventName   string             `json:"event_name"`
	StartedAt   time.Time          `json:"started_at"`
	OrderCount  int                `json:"order_count"`
	Revenue     float64            `json:"revenue"`
	ItemBreakdown map[string]ItemStats `json:"item_breakdown"`
}

// ItemStats holds stats for a single menu item
type ItemStats struct {
	ItemName string  `json:"item_name"`
	Quantity int     `json:"quantity"`
	Revenue  float64 `json:"revenue"`
}

// NewRepository creates a new repository instance
func NewRepository(dsn string) (Repository, error) {
	// TODO: Implement PostgreSQL repository
	return nil, nil
}
