-- Make vehicle_description optional
ALTER TABLE orders ALTER COLUMN vehicle_description DROP NOT NULL;
