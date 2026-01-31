-- Create sequence for order IDs
CREATE SEQUENCE IF NOT EXISTS order_id_seq START 1;

-- Menu items
CREATE TABLE menu_items (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    display_order INTEGER DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create index for faster sorting by display_order
CREATE INDEX IF NOT EXISTS idx_menu_items_display_order ON menu_items(display_order);

-- Orders
CREATE TABLE orders (
    id INTEGER PRIMARY KEY DEFAULT nextval('order_id_seq'),
    vehicle_description TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('NEW', 'IN_PROGRESS', 'COMPLETED')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Order items
CREATE TABLE order_items (
    id TEXT PRIMARY KEY,
    order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    menu_item_id TEXT NOT NULL REFERENCES menu_items(id),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Chat messages
CREATE TABLE chat_messages (
    id TEXT PRIMARY KEY,
    order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    sender_role TEXT NOT NULL CHECK (sender_role IN ('WORKER', 'ADMIN')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Initial menu items
INSERT INTO menu_items (id, name, price, is_active, display_order) VALUES
    ('baked-fish-dinner', 'Baked fish dinner', 12.99, true, 1),
    ('fried-fish-dinner', 'Fried fish dinner', 12.99, true, 2),
    ('kids-pizza-dinner', 'Kids pizza dinner', 6.99, true, 3),
    ('kids-baked-fish-dinner', 'Kids baked fish dinner', 6.99, true, 4),
    ('kids-fried-fish-dinner', 'Kids fried fish dinner', 6.99, true, 5),
    ('extra-baked-fish', 'Extra piece of baked fish', 3.99, true, 6),
    ('extra-fried-fish', 'Extra piece of fried fish', 3.99, true, 7),
    ('extra-pizza', 'Extra piece of pizza', 2.99, true, 8); 