-- Create sessions table
CREATE TABLE sessions (
    id SERIAL PRIMARY KEY,
    event_name TEXT NOT NULL,
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    closed_at TIMESTAMP,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'CLOSED')),
    final_order_count INTEGER,
    final_revenue DECIMAL(10,2),
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for sessions
CREATE INDEX idx_sessions_status ON sessions(status);
CREATE INDEX idx_sessions_started_at ON sessions(started_at);

-- Add session_id and daily_order_number to orders
ALTER TABLE orders ADD COLUMN session_id INTEGER REFERENCES sessions(id);
ALTER TABLE orders ADD COLUMN daily_order_number INTEGER;

-- Add unique constraint for daily_order_number within a session
-- (applied after migration for existing data)
-- We'll handle this constraint after data migration

-- Add price capture columns to order_items
ALTER TABLE order_items ADD COLUMN unit_price DECIMAL(10,2);
ALTER TABLE order_items ADD COLUMN item_name TEXT;

-- Make menu_item_id nullable (since we capture item_name at order time, original item may be deleted)
ALTER TABLE order_items ALTER COLUMN menu_item_id DROP NOT NULL;
ALTER TABLE order_items DROP CONSTRAINT order_items_menu_item_id_fkey;
ALTER TABLE order_items ADD CONSTRAINT order_items_menu_item_id_fkey 
    FOREIGN KEY (menu_item_id) REFERENCES menu_items(id) ON DELETE SET NULL;

-- Create index for orders by session
CREATE INDEX idx_orders_session ON orders(session_id);

-- Migrate existing orders to a default session (if any exist)
-- First, check if there are any orders
DO $$
DECLARE
    order_count INTEGER;
    new_session_id INTEGER;
BEGIN
    SELECT COUNT(*) INTO order_count FROM orders;
    
    IF order_count > 0 THEN
        -- Create a legacy session for existing orders
        INSERT INTO sessions (event_name, started_at, expires_at, status, notes, created_at, updated_at)
        VALUES (
            'Legacy Orders (Pre-Session)',
            (SELECT MIN(created_at) FROM orders),
            (SELECT MAX(created_at) FROM orders),
            'CLOSED',
            'Auto-created session for orders that existed before the session feature was added',
            CURRENT_TIMESTAMP,
            CURRENT_TIMESTAMP
        )
        RETURNING id INTO new_session_id;
        
        -- Update all existing orders to belong to this session
        UPDATE orders SET session_id = new_session_id;
        
        -- Calculate daily_order_number for existing orders
        WITH numbered_orders AS (
            SELECT id, ROW_NUMBER() OVER (ORDER BY id) as daily_num
            FROM orders
            WHERE session_id = new_session_id
        )
        UPDATE orders o
        SET daily_order_number = n.daily_num
        FROM numbered_orders n
        WHERE o.id = n.id;
        
        -- Update final stats for the legacy session
        UPDATE sessions
        SET final_order_count = (SELECT COUNT(*) FROM orders WHERE session_id = new_session_id),
            final_revenue = (
                SELECT COALESCE(SUM(mi.price * oi.quantity), 0)
                FROM order_items oi
                JOIN menu_items mi ON oi.menu_item_id = mi.id
                JOIN orders o ON oi.order_id = o.id
                WHERE o.session_id = new_session_id
            )
        WHERE id = new_session_id;
        
        -- Backfill unit_price and item_name for existing order items
        UPDATE order_items oi
        SET unit_price = mi.price,
            item_name = mi.name
        FROM menu_items mi
        WHERE oi.menu_item_id = mi.id
          AND oi.unit_price IS NULL;
    END IF;
END $$;

-- Now make session_id and daily_order_number NOT NULL (after migration)
ALTER TABLE orders ALTER COLUMN session_id SET NOT NULL;
ALTER TABLE orders ALTER COLUMN daily_order_number SET NOT NULL;

-- Make unit_price and item_name NOT NULL (after backfill)
-- Set defaults for any items that might not have menu items
UPDATE order_items SET unit_price = 0, item_name = 'Unknown Item' WHERE unit_price IS NULL;
ALTER TABLE order_items ALTER COLUMN unit_price SET NOT NULL;
ALTER TABLE order_items ALTER COLUMN item_name SET NOT NULL;

-- Add unique constraint for daily_order_number within a session
ALTER TABLE orders ADD CONSTRAINT orders_session_daily_number_unique UNIQUE (session_id, daily_order_number);
