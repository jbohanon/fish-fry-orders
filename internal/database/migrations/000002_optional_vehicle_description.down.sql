-- Make vehicle_description required again (set empty strings to placeholder first)
UPDATE orders SET vehicle_description = 'Unknown' WHERE vehicle_description IS NULL OR vehicle_description = '';
ALTER TABLE orders ALTER COLUMN vehicle_description SET NOT NULL;
