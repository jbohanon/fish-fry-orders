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
	// Sort by: status priority (IN_PROGRESS first, then NEW, then COMPLETED), then by ID ascending
	query := `
		SELECT id, vehicle_description, status, created_at, updated_at 
		FROM orders 
		ORDER BY 
			CASE status 
				WHEN 'IN_PROGRESS' THEN 1
				WHEN 'NEW' THEN 2
				WHEN 'COMPLETED' THEN 3
				ELSE 4
			END,
			id ASC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}
	defer rows.Close()

	var orders []types.DBOrder
	for rows.Next() {
		var order types.DBOrder
		if err := rows.Scan(&order.ID, &order.VehicleDescription, &order.Status, &order.CreatedAt, &order.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, order)
	}

	return orders, nil
}

func (r *PostgresRepository) GetOrderByID(ctx context.Context, id int) (*types.DBOrder, error) {
	query := `SELECT id, vehicle_description, status, created_at, updated_at FROM orders WHERE id = $1`
	var order types.DBOrder
	if err := r.pool.QueryRow(ctx, query, id).Scan(&order.ID, &order.VehicleDescription, &order.Status, &order.CreatedAt, &order.UpdatedAt); err != nil {
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

	query := `INSERT INTO orders (vehicle_description, status, created_at, updated_at) VALUES ($1, $2, $3, $4) RETURNING id`
	err := r.pool.QueryRow(ctx, query, order.VehicleDescription, order.Status, order.CreatedAt, order.UpdatedAt).Scan(&order.ID)
	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
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
	query := `SELECT id, order_id, menu_item_id, quantity, created_at FROM order_items WHERE order_id = $1 ORDER BY created_at`
	rows, err := r.pool.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order items: %w", err)
	}
	defer rows.Close()

	var items []types.DBOrderItem
	for rows.Next() {
		var item types.DBOrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.MenuItemID, &item.Quantity, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan order item: %w", err)
		}
		items = append(items, item)
	}

	return items, nil
}

func (r *PostgresRepository) CreateOrderItem(ctx context.Context, item *types.DBOrderItem) error {
	item.ID = uuid.New().String()
	item.CreatedAt = time.Now()

	query := `INSERT INTO order_items (id, order_id, menu_item_id, quantity, created_at) VALUES ($1, $2, $3, $4, $5)`
	_, err := r.pool.Exec(ctx, query, item.ID, item.OrderID, item.MenuItemID, item.Quantity, item.CreatedAt)
	if err != nil {
		logger.ErrorWithErr("Database error: failed to create order item", err,
			"order_id", item.OrderID,
			"menu_item_id", item.MenuItemID,
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

// Chat messages

func (r *PostgresRepository) GetChatMessages(ctx context.Context, orderID int) ([]types.DBChatMessage, error) {
	query := `SELECT id, order_id, content, sender_role, created_at FROM chat_messages WHERE order_id = $1 ORDER BY created_at`
	rows, err := r.pool.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat messages: %w", err)
	}
	defer rows.Close()

	var messages []types.DBChatMessage
	for rows.Next() {
		var message types.DBChatMessage
		if err := rows.Scan(&message.ID, &message.OrderID, &message.Content, &message.SenderRole, &message.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan chat message: %w", err)
		}
		messages = append(messages, message)
	}

	return messages, nil
}

func (r *PostgresRepository) CreateChatMessage(ctx context.Context, message *types.DBChatMessage) error {
	message.ID = uuid.New().String()
	message.CreatedAt = time.Now()

	query := `INSERT INTO chat_messages (id, order_id, content, sender_role, created_at) VALUES ($1, $2, $3, $4, $5)`
	_, err := r.pool.Exec(ctx, query, message.ID, message.OrderID, message.Content, message.SenderRole, message.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create chat message: %w", err)
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
