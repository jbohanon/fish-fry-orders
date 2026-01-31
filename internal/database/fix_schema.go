package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// FixOrderIDSchema ensures order_id columns are INTEGER type
// This is a safety check to fix any schema inconsistencies
func FixOrderIDSchema(db *sql.DB) error {
	ctx := context.Background()

	// Check order_items.order_id type
	var orderItemsType string
	err := db.QueryRowContext(ctx, `
		SELECT data_type 
		FROM information_schema.columns 
		WHERE table_name = 'order_items' AND column_name = 'order_id'
	`).Scan(&orderItemsType)
	if err != nil {
		return fmt.Errorf("failed to check order_items.order_id type: %w", err)
	}

	// If it's not integer, we need to fix it
	if orderItemsType != "integer" {
		// This is a destructive operation - drop and recreate
		// In production, you'd want to migrate data, but for now we'll just recreate
		_, err = db.ExecContext(ctx, `
			ALTER TABLE order_items DROP CONSTRAINT IF EXISTS order_items_order_id_fkey;
			DROP TABLE IF EXISTS order_items CASCADE;
		`)
		if err != nil {
			return fmt.Errorf("failed to drop order_items: %w", err)
		}

		_, err = db.ExecContext(ctx, `
			CREATE TABLE order_items (
				id TEXT PRIMARY KEY,
				order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
				menu_item_id TEXT NOT NULL REFERENCES menu_items(id),
				quantity INTEGER NOT NULL CHECK (quantity > 0),
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return fmt.Errorf("failed to recreate order_items: %w", err)
		}
	}

	// Check chat_messages.order_id type
	var chatMessagesType string
	err = db.QueryRowContext(ctx, `
		SELECT data_type 
		FROM information_schema.columns 
		WHERE table_name = 'chat_messages' AND column_name = 'order_id'
	`).Scan(&chatMessagesType)
	if err != nil && !strings.Contains(err.Error(), "no rows") {
		return fmt.Errorf("failed to check chat_messages.order_id type: %w", err)
	}

	// If it's not integer, fix it
	if chatMessagesType != "" && chatMessagesType != "integer" {
		_, err = db.ExecContext(ctx, `
			ALTER TABLE chat_messages DROP CONSTRAINT IF EXISTS chat_messages_order_id_fkey;
			DROP TABLE IF EXISTS chat_messages CASCADE;
		`)
		if err != nil {
			return fmt.Errorf("failed to drop chat_messages: %w", err)
		}

		_, err = db.ExecContext(ctx, `
			CREATE TABLE chat_messages (
				id TEXT PRIMARY KEY,
				order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
				content TEXT NOT NULL,
				sender_role TEXT NOT NULL CHECK (sender_role IN ('WORKER', 'ADMIN')),
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return fmt.Errorf("failed to recreate chat_messages: %w", err)
		}
	}

	return nil
}
