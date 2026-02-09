package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"git.nonahob.net/jacob/fish-fry-orders/internal/config"
	"git.nonahob.net/jacob/fish-fry-orders/proto"
	"github.com/redis/go-redis/v9"
)

type Database struct {
	pool *pgxpool.Pool
}

type RedisClient struct {
	client *redis.Client
}

func New(cfg *config.DatabaseConfig) (*Database, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %v", err)
	}

	return &Database{pool: pool}, nil
}

func (db *Database) Close() {
	db.pool.Close()
}

func NewRedisClient(cfg *config.RedisConfig) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	return &RedisClient{client: client}, nil
}

func (r *RedisClient) Close() {
	r.client.Close()
}

// Database operations
func (db *Database) CreateOrder(ctx context.Context, order *proto.Order) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Insert order
	_, err = tx.Exec(ctx, `
		INSERT INTO orders (id, vehicle_description, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`, order.Id, order.VehicleDescription, order.Status, order.CreatedAt, order.UpdatedAt)
	if err != nil {
		return err
	}

	// Insert order items
	for _, item := range order.Items {
		_, err = tx.Exec(ctx, `
			INSERT INTO order_items (order_id, menu_item_id, quantity)
			VALUES ($1, $2, $3)
		`, order.Id, item.MenuItemId, item.Quantity)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (db *Database) UpdateOrderStatus(ctx context.Context, orderID string, status proto.OrderStatus) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE orders
		SET status = $1, updated_at = $2
		WHERE id = $3
	`, status, time.Now().Unix(), orderID)
	return err
}

func (db *Database) GetOrders(ctx context.Context, statusFilter string) ([]*proto.Order, error) {
	var query string
	var args []interface{}

	if statusFilter != "" {
		query = `
			SELECT o.id, o.vehicle_description, o.status, o.created_at, o.updated_at,
				   oi.menu_item_id, oi.quantity
			FROM orders o
			LEFT JOIN order_items oi ON o.id = oi.order_id
			WHERE o.status = $1
			ORDER BY o.created_at DESC
		`
		args = []interface{}{statusFilter}
	} else {
		query = `
			SELECT o.id, o.vehicle_description, o.status, o.created_at, o.updated_at,
				   oi.menu_item_id, oi.quantity
			FROM orders o
			LEFT JOIN order_items oi ON o.id = oi.order_id
			ORDER BY o.created_at DESC
		`
	}

	rows, err := db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make(map[string]*proto.Order)
	for rows.Next() {
		var orderID, vehicleDesc string
		var status int32
		var createdAt, updatedAt int64
		var menuItemID string
		var quantity int32

		err := rows.Scan(&orderID, &vehicleDesc, &status, &createdAt, &updatedAt, &menuItemID, &quantity)
		if err != nil {
			return nil, err
		}

		order, exists := orders[orderID]
		if !exists {
			order = &proto.Order{
				Id:                 orderID,
				VehicleDescription: vehicleDesc,
				Status:             proto.OrderStatus(status),
				CreatedAt:          createdAt,
				UpdatedAt:          updatedAt,
				Items:              make([]*proto.OrderItem, 0),
			}
			orders[orderID] = order
		}

		if menuItemID != "" {
			order.Items = append(order.Items, &proto.OrderItem{
				MenuItemId: menuItemID,
				Quantity:   quantity,
			})
		}
	}

	result := make([]*proto.Order, 0, len(orders))
	for _, order := range orders {
		result = append(result, order)
	}

	return result, nil
}

func (d *Database) GetOrder(ctx context.Context, orderID string) (*proto.Order, error) {
	// TODO: Implement single order retrieval
	return nil, nil
}

func (d *Database) CreateMessage(ctx context.Context, message *proto.ChatMessage) error {
	// TODO: Implement message creation
	return nil
}

func (d *Database) GetMessages(ctx context.Context, orderID string) ([]*proto.ChatMessage, error) {
	// TODO: Implement message retrieval
	return nil, nil
}

func (d *Database) GetStatistics(ctx context.Context) (*proto.OrderStatistics, error) {
	// TODO: Implement statistics retrieval
	return nil, nil
}

func (d *Database) GetMenuItems(ctx context.Context) ([]*proto.MenuItem, error) {
	// TODO: Implement menu items retrieval
	return nil, nil
}

func (d *Database) UpdateMenuItem(ctx context.Context, item *proto.MenuItem) error {
	// TODO: Implement menu item update
	return nil
}
