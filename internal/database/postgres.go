package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jbohanon/fish-fry-orders-v2/internal/logger"
	"github.com/jbohanon/fish-fry-orders-v2/internal/types"
)

// PostgresRepository implements the Repository interface using PostgreSQL
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository using a connection pool
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Sessions

func (r *PostgresRepository) GetActiveSession(ctx context.Context) (*types.DBSession, error) {
	query := `
		SELECT id, event_name, started_at, expires_at, closed_at, status, 
		       final_order_count, final_revenue, notes, created_at, updated_at
		FROM sessions 
		WHERE status = 'ACTIVE' AND expires_at > NOW()
		ORDER BY started_at DESC
		LIMIT 1
	`
	var session types.DBSession
	err := r.pool.QueryRow(ctx, query).Scan(
		&session.ID, &session.EventName, &session.StartedAt, &session.ExpiresAt,
		&session.ClosedAt, &session.Status, &session.FinalOrderCount, &session.FinalRevenue,
		&session.Notes, &session.CreatedAt, &session.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}
	return &session, nil
}

func (r *PostgresRepository) GetSessionByID(ctx context.Context, id int) (*types.DBSession, error) {
	query := `
		SELECT id, event_name, started_at, expires_at, closed_at, status, 
		       final_order_count, final_revenue, notes, created_at, updated_at
		FROM sessions 
		WHERE id = $1
	`
	var session types.DBSession
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&session.ID, &session.EventName, &session.StartedAt, &session.ExpiresAt,
		&session.ClosedAt, &session.Status, &session.FinalOrderCount, &session.FinalRevenue,
		&session.Notes, &session.CreatedAt, &session.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return &session, nil
}

func (r *PostgresRepository) GetSessions(ctx context.Context, from, to *time.Time) ([]types.DBSession, error) {
	query := `
		SELECT id, event_name, started_at, expires_at, closed_at, status, 
		       final_order_count, final_revenue, notes, created_at, updated_at
		FROM sessions 
		WHERE ($1::timestamp IS NULL OR started_at >= $1)
		  AND ($2::timestamp IS NULL OR started_at <= $2)
		ORDER BY started_at DESC
	`
	rows, err := r.pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}
	defer rows.Close()

	var sessions []types.DBSession
	for rows.Next() {
		var session types.DBSession
		if err := rows.Scan(
			&session.ID, &session.EventName, &session.StartedAt, &session.ExpiresAt,
			&session.ClosedAt, &session.Status, &session.FinalOrderCount, &session.FinalRevenue,
			&session.Notes, &session.CreatedAt, &session.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (r *PostgresRepository) CreateSession(ctx context.Context, session *types.DBSession) error {
	session.CreatedAt = time.Now()
	session.UpdatedAt = session.CreatedAt
	if session.Status == "" {
		session.Status = "ACTIVE"
	}

	query := `
		INSERT INTO sessions (event_name, started_at, expires_at, status, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	err := r.pool.QueryRow(ctx, query,
		session.EventName, session.StartedAt, session.ExpiresAt, session.Status,
		session.Notes, session.CreatedAt, session.UpdatedAt,
	).Scan(&session.ID)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

func (r *PostgresRepository) UpdateSession(ctx context.Context, session *types.DBSession) error {
	session.UpdatedAt = time.Now()

	query := `
		UPDATE sessions 
		SET event_name = $1, expires_at = $2, status = $3, notes = $4, updated_at = $5
		WHERE id = $6
	`
	_, err := r.pool.Exec(ctx, query,
		session.EventName, session.ExpiresAt, session.Status, session.Notes,
		session.UpdatedAt, session.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}
	return nil
}

func (r *PostgresRepository) CloseSession(ctx context.Context, sessionID int) error {
	// Get current stats
	orderCount, revenue, err := r.GetSessionStats(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session stats: %w", err)
	}

	now := time.Now()
	query := `
		UPDATE sessions 
		SET status = 'CLOSED', closed_at = $1, final_order_count = $2, final_revenue = $3, updated_at = $1
		WHERE id = $4
	`
	_, err = r.pool.Exec(ctx, query, now, orderCount, revenue, sessionID)
	if err != nil {
		return fmt.Errorf("failed to close session: %w", err)
	}

	// Mark all incomplete orders as completed
	err = r.CompleteAllSessionOrders(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to complete session orders: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetOrCreateActiveSession(ctx context.Context) (*types.DBSession, error) {
	// Try to get existing active session
	session, err := r.GetActiveSession(ctx)
	if err != nil {
		return nil, err
	}
	if session != nil {
		return session, nil
	}

	// Create new session
	now := time.Now()
	// Default expiry is end of today (midnight)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())

	session = &types.DBSession{
		EventName: fmt.Sprintf("Fish Fry %s", now.Format("2006-01-02")),
		StartedAt: now,
		ExpiresAt: endOfDay,
		Status:    "ACTIVE",
	}

	if err := r.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	logger.Info("Auto-created new session", "session_id", session.ID, "event_name", session.EventName)
	return session, nil
}

func (r *PostgresRepository) GetNextDailyOrderNumber(ctx context.Context, sessionID int) (int, error) {
	var nextNum int
	query := `SELECT COALESCE(MAX(daily_order_number), 0) + 1 FROM orders WHERE session_id = $1`
	err := r.pool.QueryRow(ctx, query, sessionID).Scan(&nextNum)
	if err != nil {
		return 0, fmt.Errorf("failed to get next daily order number: %w", err)
	}
	return nextNum, nil
}

func (r *PostgresRepository) GetSessionStats(ctx context.Context, sessionID int) (orderCount int, revenue float64, err error) {
	query := `
		SELECT 
			COUNT(DISTINCT o.id)::int,
			COALESCE(SUM(oi.unit_price * oi.quantity), 0)
		FROM orders o
		LEFT JOIN order_items oi ON o.id = oi.order_id
		WHERE o.session_id = $1
	`
	err = r.pool.QueryRow(ctx, query, sessionID).Scan(&orderCount, &revenue)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get session stats: %w", err)
	}
	return orderCount, revenue, nil
}

func (r *PostgresRepository) CompareSessionStats(ctx context.Context, sessionIDs []int) ([]SessionComparisonStats, error) {
	results := make([]SessionComparisonStats, 0, len(sessionIDs))

	for _, sessionID := range sessionIDs {
		session, err := r.GetSessionByID(ctx, sessionID)
		if err != nil || session == nil {
			continue
		}

		orderCount, revenue, err := r.GetSessionStats(ctx, sessionID)
		if err != nil {
			continue
		}

		// Get item breakdown
		itemQuery := `
			SELECT oi.item_name, SUM(oi.quantity)::int, SUM(oi.unit_price * oi.quantity)
			FROM order_items oi
			JOIN orders o ON oi.order_id = o.id
			WHERE o.session_id = $1
			GROUP BY oi.item_name
			ORDER BY SUM(oi.unit_price * oi.quantity) DESC
		`
		rows, err := r.pool.Query(ctx, itemQuery, sessionID)
		if err != nil {
			continue
		}

		itemBreakdown := make(map[string]ItemStats)
		for rows.Next() {
			var itemName string
			var quantity int
			var itemRevenue float64
			if err := rows.Scan(&itemName, &quantity, &itemRevenue); err != nil {
				continue
			}
			itemBreakdown[itemName] = ItemStats{
				ItemName: itemName,
				Quantity: quantity,
				Revenue:  itemRevenue,
			}
		}
		rows.Close()

		results = append(results, SessionComparisonStats{
			SessionID:     sessionID,
			EventName:     session.EventName,
			StartedAt:     session.StartedAt,
			OrderCount:    orderCount,
			Revenue:       revenue,
			ItemBreakdown: itemBreakdown,
		})
	}

	return results, nil
}

func (r *PostgresRepository) CompleteAllSessionOrders(ctx context.Context, sessionID int) error {
	query := `
		UPDATE orders 
		SET status = 'COMPLETED', updated_at = NOW()
		WHERE session_id = $1 AND status != 'COMPLETED'
	`
	_, err := r.pool.Exec(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to complete session orders: %w", err)
	}
	return nil
}

// Menu items

func (r *PostgresRepository) GetMenuItems(ctx context.Context) ([]types.DBMenuItem, error) {
	query := `SELECT id, name, price, is_active, display_order, created_at, updated_at FROM menu_items ORDER BY display_order, name`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get menu items: %w", err)
	}
	defer rows.Close()

	var items []types.DBMenuItem
	for rows.Next() {
		var item types.DBMenuItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Price, &item.IsActive, &item.DisplayOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan menu item: %w", err)
		}
		items = append(items, item)
	}

	return items, nil
}

func (r *PostgresRepository) GetMenuItemByID(ctx context.Context, id string) (*types.DBMenuItem, error) {
	query := `SELECT id, name, price, is_active, display_order, created_at, updated_at FROM menu_items WHERE id = $1`
	var item types.DBMenuItem
	if err := r.pool.QueryRow(ctx, query, id).Scan(&item.ID, &item.Name, &item.Price, &item.IsActive, &item.DisplayOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get menu item: %w", err)
	}
	return &item, nil
}

func (r *PostgresRepository) CreateMenuItem(ctx context.Context, item *types.DBMenuItem) error {
	item.ID = uuid.New().String()
	item.CreatedAt = time.Now()
	item.UpdatedAt = item.CreatedAt
	
	// If display_order is not set, get the max and add 1
	if item.DisplayOrder == 0 {
		var maxOrder int
		err := r.pool.QueryRow(ctx, `SELECT COALESCE(MAX(display_order), 0) FROM menu_items`).Scan(&maxOrder)
		if err != nil {
			return fmt.Errorf("failed to get max display order: %w", err)
		}
		item.DisplayOrder = maxOrder + 1
	}

	query := `INSERT INTO menu_items (id, name, price, is_active, display_order, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.pool.Exec(ctx, query, item.ID, item.Name, item.Price, item.IsActive, item.DisplayOrder, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create menu item: %w", err)
	}
	return nil
}

func (r *PostgresRepository) UpdateMenuItem(ctx context.Context, item *types.DBMenuItem) error {
	item.UpdatedAt = time.Now()

	query := `UPDATE menu_items SET name = $1, price = $2, is_active = $3, display_order = $4, updated_at = $5 WHERE id = $6`
	_, err := r.pool.Exec(ctx, query, item.Name, item.Price, item.IsActive, item.DisplayOrder, item.UpdatedAt, item.ID)
	if err != nil {
		return fmt.Errorf("failed to update menu item: %w", err)
	}
	return nil
}

func (r *PostgresRepository) DeleteMenuItem(ctx context.Context, id string) error {
	query := `DELETE FROM menu_items WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete menu item: %w", err)
	}
	return nil
}

func (r *PostgresRepository) UpdateMenuItemsOrder(ctx context.Context, itemOrders map[string]int) error {
	// Use a transaction to update all items atomically
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for itemID, displayOrder := range itemOrders {
		_, err := tx.Exec(ctx, `UPDATE menu_items SET display_order = $1, updated_at = $2 WHERE id = $3`,
			displayOrder, time.Now(), itemID)
		if err != nil {
			return fmt.Errorf("failed to update menu item order for %s: %w", itemID, err)
		}
	}

	return tx.Commit(ctx)
}

// Orders

func (r *PostgresRepository) GetOrders(ctx context.Context) ([]types.DBOrder, error) {
	// Get orders for the current active session only
	session, err := r.GetActiveSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}
	if session == nil {
		// No active session, return empty list
		return []types.DBOrder{}, nil
	}
	return r.GetOrdersBySession(ctx, session.ID)
}

func (r *PostgresRepository) GetOrdersBySession(ctx context.Context, sessionID int) ([]types.DBOrder, error) {
	// Sort by: status priority (IN_PROGRESS first, then NEW, then COMPLETED), then by daily_order_number ascending
	query := `
		SELECT id, session_id, daily_order_number, vehicle_description, status, created_at, updated_at
		FROM orders 
		WHERE session_id = $1
		ORDER BY 
			CASE status 
				WHEN 'IN_PROGRESS' THEN 1
				WHEN 'NEW' THEN 2
				WHEN 'COMPLETED' THEN 3
				ELSE 4
			END,
			daily_order_number ASC
	`
	rows, err := r.pool.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}
	defer rows.Close()

	var orders []types.DBOrder
	for rows.Next() {
		var order types.DBOrder
		if err := rows.Scan(&order.ID, &order.SessionID, &order.DailyOrderNumber, &order.VehicleDescription, &order.Status, &order.CreatedAt, &order.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, order)
	}

	return orders, nil
}

func (r *PostgresRepository) GetOrderByID(ctx context.Context, id int) (*types.DBOrder, error) {
	query := `
		SELECT id, session_id, daily_order_number, vehicle_description, status, created_at, updated_at
		FROM orders
		WHERE id = $1
	`
	var order types.DBOrder
	if err := r.pool.QueryRow(ctx, query, id).Scan(&order.ID, &order.SessionID, &order.DailyOrderNumber, &order.VehicleDescription, &order.Status, &order.CreatedAt, &order.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}
	return &order, nil
}

func (r *PostgresRepository) GetNextOrderID(ctx context.Context) (int, error) {
	// Get the next ID without incrementing the sequence
	// We check both the sequence's last_value and the actual max order ID
	// and use whichever is higher, then add 1
	var nextID int
	query := `
		SELECT GREATEST(
			COALESCE((SELECT last_value FROM order_id_seq), 0),
			COALESCE((SELECT MAX(id) FROM orders), 0)
		) + 1
	`
	if err := r.pool.QueryRow(ctx, query).Scan(&nextID); err != nil {
		return 0, fmt.Errorf("failed to get next order ID: %w", err)
	}
	return nextID, nil
}

func (r *PostgresRepository) CreateOrder(ctx context.Context, order *types.DBOrder) error {
	order.CreatedAt = time.Now()
	order.UpdatedAt = order.CreatedAt

	query := `INSERT INTO orders (session_id, daily_order_number, vehicle_description, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	err := r.pool.QueryRow(ctx, query, order.SessionID, order.DailyOrderNumber, order.VehicleDescription, order.Status, order.CreatedAt, order.UpdatedAt).Scan(&order.ID)
	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}
	return nil
}

// CreateOrderWithItems creates an order and its items atomically in a transaction.
// It assigns the daily order number using an advisory lock to prevent race conditions.
func (r *PostgresRepository) CreateOrderWithItems(ctx context.Context, order *types.DBOrder, items []*types.DBOrderItem) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now()
	order.CreatedAt = now
	order.UpdatedAt = now

	// Acquire advisory lock on the session ID to prevent concurrent order creation race conditions
	// This lock is released when the transaction ends
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, order.SessionID)
	if err != nil {
		return fmt.Errorf("failed to acquire advisory lock: %w", err)
	}

	// Now safely get the next daily order number
	var nextNum int
	err = tx.QueryRow(ctx, `SELECT COALESCE(MAX(daily_order_number), 0) + 1 FROM orders WHERE session_id = $1`, order.SessionID).Scan(&nextNum)
	if err != nil {
		return fmt.Errorf("failed to get next daily order number: %w", err)
	}
	order.DailyOrderNumber = nextNum

	// Create the order
	orderQuery := `
		INSERT INTO orders (session_id, daily_order_number, vehicle_description, status, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6) 
		RETURNING id
	`
	err = tx.QueryRow(ctx, orderQuery,
		order.SessionID, order.DailyOrderNumber, order.VehicleDescription, order.Status, order.CreatedAt, order.UpdatedAt,
	).Scan(&order.ID)
	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	// Create all order items
	itemQuery := `INSERT INTO order_items (id, order_id, menu_item_id, item_name, unit_price, quantity, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	for _, item := range items {
		item.ID = uuid.New().String()
		item.OrderID = order.ID
		item.CreatedAt = now

		_, err = tx.Exec(ctx, itemQuery, item.ID, item.OrderID, item.MenuItemID, item.ItemName, item.UnitPrice, item.Quantity, item.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to create order item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *PostgresRepository) UpdateOrder(ctx context.Context, order *types.DBOrder) error {
	order.UpdatedAt = time.Now()

	query := `UPDATE orders SET vehicle_description = $1, status = $2, updated_at = $3 WHERE id = $4`
	_, err := r.pool.Exec(ctx, query, order.VehicleDescription, order.Status, order.UpdatedAt, order.ID)
	if err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}
	return nil
}

func (r *PostgresRepository) DeleteOrder(ctx context.Context, id int) error {
	query := `DELETE FROM orders WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete order: %w", err)
	}
	return nil
}

// Order items

func (r *PostgresRepository) GetOrderItems(ctx context.Context, orderID int) ([]types.DBOrderItem, error) {
	query := `SELECT id, order_id, menu_item_id, item_name, unit_price, quantity, created_at FROM order_items WHERE order_id = $1 ORDER BY created_at`
	rows, err := r.pool.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order items: %w", err)
	}
	defer rows.Close()

	var items []types.DBOrderItem
	for rows.Next() {
		var item types.DBOrderItem
		var menuItemID *string
		if err := rows.Scan(&item.ID, &item.OrderID, &menuItemID, &item.ItemName, &item.UnitPrice, &item.Quantity, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}
		if menuItemID != nil {
			item.MenuItemID = *menuItemID
		}
		items = append(items, item)
	}

	return items, nil
}

func (r *PostgresRepository) CreateOrderItem(ctx context.Context, item *types.DBOrderItem) error {
	item.ID = uuid.New().String()
	item.CreatedAt = time.Now()

	query := `INSERT INTO order_items (id, order_id, menu_item_id, item_name, unit_price, quantity, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.pool.Exec(ctx, query, item.ID, item.OrderID, item.MenuItemID, item.ItemName, item.UnitPrice, item.Quantity, item.CreatedAt)
	if err != nil {
		logger.ErrorWithErr("Database error: failed to create order item", err,
			"order_id", item.OrderID,
			"menu_item_id", item.MenuItemID,
			"item_name", item.ItemName,
			"unit_price", item.UnitPrice,
			"quantity", item.Quantity,
		)
		return fmt.Errorf("failed to create order item: %w", err)
	}
	return nil
}

func (r *PostgresRepository) UpdateOrderItem(ctx context.Context, item *types.DBOrderItem) error {
	query := `UPDATE order_items SET quantity = $1 WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, item.Quantity, item.ID)
	if err != nil {
		return fmt.Errorf("failed to update order item: %w", err)
	}
	return nil
}

func (r *PostgresRepository) DeleteOrderItem(ctx context.Context, id string) error {
	query := `DELETE FROM order_items WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete order item: %w", err)
	}
	return nil
}

// Statistics

func (r *PostgresRepository) GetOrderStatistics(ctx context.Context, startTime, endTime time.Time) (*types.DBOrderStatistics, error) {
	stats := &types.DBOrderStatistics{
		ItemCounts: make(map[string]int32),
	}

	// Get item counts
	rows, err := r.pool.Query(ctx, `
		SELECT mi.name, COUNT(oi.id)::int4
		FROM order_items oi
		JOIN menu_items mi ON oi.menu_item_id = mi.id
		JOIN orders o ON oi.order_id = o.id
		WHERE o.created_at BETWEEN $1 AND $2
		GROUP BY mi.name
	`, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get item counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var count int32
		if err := rows.Scan(&name, &count); err != nil {
			return nil, fmt.Errorf("failed to scan item count: %w", err)
		}
		stats.ItemCounts[name] = count
	}

	// Get average completion time
	var avgTime float64
	err = r.pool.QueryRow(ctx, `
		SELECT AVG(EXTRACT(EPOCH FROM (updated_at - created_at)))
		FROM orders
		WHERE status = 'COMPLETED'
		AND created_at BETWEEN $1 AND $2
	`, startTime, endTime).Scan(&avgTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get average completion time: %w", err)
	}
	stats.AverageTimeToComplete = time.Duration(avgTime * float64(time.Second))

	// Get total orders
	var totalOrders int32
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)::int4
		FROM orders
		WHERE created_at BETWEEN $1 AND $2
	`, startTime, endTime).Scan(&totalOrders)
	if err != nil {
		return nil, fmt.Errorf("failed to get total orders: %w", err)
	}
	stats.TotalOrders = totalOrders

	return stats, nil
}

// PurgeOrdersToday deletes all orders created today and returns the count of deleted orders
func (r *PostgresRepository) PurgeOrdersToday(ctx context.Context) (int, error) {
	today := time.Now()
	startOfToday := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	endOfToday := startOfToday.Add(24 * time.Hour)

	// Get count before deletion
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)::int4
		FROM orders
		WHERE created_at >= $1 AND created_at < $2
	`, startOfToday, endOfToday).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count orders: %w", err)
	}

	// Delete orders (cascade will handle order_items and chat_messages)
	_, err = r.pool.Exec(ctx, `
		DELETE FROM orders
		WHERE created_at >= $1 AND created_at < $2
	`, startOfToday, endOfToday)
	if err != nil {
		return 0, fmt.Errorf("failed to purge orders: %w", err)
	}

	return count, nil
}

// PurgeAllOrders deletes all orders and returns the count of deleted orders
func (r *PostgresRepository) PurgeAllOrders(ctx context.Context) (int, error) {
	// Get count before deletion
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*)::int4 FROM orders`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count orders: %w", err)
	}

	// Delete all orders (cascade will handle order_items and chat_messages)
	_, err = r.pool.Exec(ctx, `DELETE FROM orders`)
	if err != nil {
		return 0, fmt.Errorf("failed to purge all orders: %w", err)
	}

	return count, nil
}
