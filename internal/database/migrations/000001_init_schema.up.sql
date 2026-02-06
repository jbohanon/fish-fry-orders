-- =============================================================================
-- Fish Fry Orders - Initial Schema
-- =============================================================================

-- Sessions table (for daily operations)
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

CREATE INDEX idx_sessions_status ON sessions(status);
CREATE INDEX idx_sessions_started_at ON sessions(started_at);

-- Menu items
CREATE TABLE menu_items (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name TEXT NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    display_order INTEGER DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_menu_items_display_order ON menu_items(display_order);

-- Orders
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    session_id INTEGER NOT NULL REFERENCES sessions(id),
    daily_order_number INTEGER NOT NULL,
    vehicle_description TEXT,
    status TEXT NOT NULL CHECK (status IN ('NEW', 'IN_PROGRESS', 'COMPLETED')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT orders_session_daily_number_unique UNIQUE (session_id, daily_order_number)
);

CREATE INDEX idx_orders_session ON orders(session_id);

-- Order items (captures price at time of order)
CREATE TABLE order_items (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    menu_item_id TEXT REFERENCES menu_items(id) ON DELETE SET NULL,
    item_name TEXT NOT NULL,
    unit_price DECIMAL(10,2) NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Chat messages
CREATE TABLE chat_messages (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    sender_role TEXT NOT NULL CHECK (sender_role IN ('WORKER', 'ADMIN')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- =============================================================================
-- Default Menu Items
-- =============================================================================

INSERT INTO menu_items (id, name, price, is_active, display_order) VALUES
    (gen_random_uuid()::text, 'Fried Fish Meal', 10.00, true, 1),
    (gen_random_uuid()::text, 'Baked Fish Meal', 10.00, true, 2),
    (gen_random_uuid()::text, 'Kid''s Cheese Pizza Meal', 4.00, true, 3),
    (gen_random_uuid()::text, 'Kid''s Fried Fish Meal', 4.00, true, 4),
    (gen_random_uuid()::text, 'Kid''s Baked Fish Meal', 4.00, true, 5),
    (gen_random_uuid()::text, 'Extra Piece of Fried Fish', 2.00, true, 6),
    (gen_random_uuid()::text, 'Extra Piece of Baked Fish', 2.00, true, 7),
    (gen_random_uuid()::text, 'Extra Pizza Slice', 1.00, true, 8);
