-- Remove unique constraint
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_session_daily_number_unique;

-- Remove NOT NULL constraints
ALTER TABLE orders ALTER COLUMN session_id DROP NOT NULL;
ALTER TABLE orders ALTER COLUMN daily_order_number DROP NOT NULL;
ALTER TABLE order_items ALTER COLUMN unit_price DROP NOT NULL;
ALTER TABLE order_items ALTER COLUMN item_name DROP NOT NULL;

-- Drop indexes
DROP INDEX IF EXISTS idx_orders_session;

-- Remove columns from order_items
ALTER TABLE order_items DROP COLUMN IF EXISTS unit_price;
ALTER TABLE order_items DROP COLUMN IF EXISTS item_name;

-- Restore menu_item_id NOT NULL constraint
ALTER TABLE order_items DROP CONSTRAINT IF EXISTS order_items_menu_item_id_fkey;
ALTER TABLE order_items ADD CONSTRAINT order_items_menu_item_id_fkey 
    FOREIGN KEY (menu_item_id) REFERENCES menu_items(id);
-- Note: This may fail if there are orphaned items; manual cleanup may be needed

-- Remove columns from orders
ALTER TABLE orders DROP COLUMN IF EXISTS session_id;
ALTER TABLE orders DROP COLUMN IF EXISTS daily_order_number;

-- Drop sessions indexes
DROP INDEX IF EXISTS idx_sessions_status;
DROP INDEX IF EXISTS idx_sessions_started_at;

-- Drop sessions table
DROP TABLE IF EXISTS sessions;
