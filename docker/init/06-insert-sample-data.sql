-- 06-insert-sample-data.sql
-- Insert sample data for testing pg-genrole functionality

\c app_db;

SET search_path TO app_schema, public;

-- Temporarily disable triggers for data loading
SET session_replication_role = replica;

-- Insert categories (hierarchical structure)
INSERT INTO categories (name, description, parent_id, is_active, sort_order) VALUES
('Electronics', 'Electronic devices and accessories', NULL, TRUE, 1),
('Computers', 'Desktop and laptop computers', 1, TRUE, 10),
('Laptops', 'Portable computers', 2, TRUE, 11),
('Desktops', 'Desktop computers', 2, TRUE, 12),
('Mobile Devices', 'Smartphones and tablets', 1, TRUE, 20),
('Smartphones', 'Mobile phones', 5, TRUE, 21),
('Tablets', 'Tablet computers', 5, TRUE, 22),
('Accessories', 'Electronic accessories', 1, TRUE, 30),
('Audio', 'Audio equipment and headphones', 8, TRUE, 31),
('Cables', 'Cables and adapters', 8, TRUE, 32),
('Clothing', 'Apparel and fashion', NULL, TRUE, 2),
('Men', 'Men''s clothing', 11, TRUE, 40),
('Women', 'Women''s clothing', 11, TRUE, 41),
('Shoes', 'Footwear for all', 11, TRUE, 42),
('Home & Garden', 'Home improvement and garden supplies', NULL, TRUE, 3),
('Furniture', 'Home furniture', 15, TRUE, 50),
('Garden', 'Garden tools and plants', 15, TRUE, 51),
('Books', 'Books and educational materials', NULL, TRUE, 4),
('Fiction', 'Fiction books', 18, TRUE, 60),
('Non-Fiction', 'Non-fiction books', 18, TRUE, 61),
('Sports', 'Sports equipment and gear', NULL, FALSE, 5);

-- Insert sample users
INSERT INTO users (username, email, password_hash, status, address, metadata) VALUES
('admin_user', 'admin@example.com', crypt('admin_password', gen_salt('bf')), 'active', 
 ROW('123 Admin St', 'Admin City', 'CA', '90210'), 
 '{"role": "admin", "department": "IT", "permissions": ["all"]}'),
 
('john_doe', 'john.doe@example.com', crypt('user_password', gen_salt('bf')), 'active',
 ROW('456 User Ave', 'User City', 'NY', '10001'),
 '{"preferences": {"theme": "dark", "notifications": true}, "age": 30}'),
 
('jane_smith', 'jane.smith@example.com', crypt('user_password', gen_salt('bf')), 'active',
 ROW('789 Customer Blvd', 'Customer Town', 'TX', '75001'),
 '{"preferences": {"theme": "light", "notifications": false}, "age": 28}'),
 
('bob_wilson', 'bob.wilson@example.com', crypt('user_password', gen_salt('bf')), 'pending',
 ROW('321 Pending Rd', 'Pending City', 'FL', '33101'),
 '{"signup_source": "referral", "referrer_id": 2}'),
 
('alice_brown', 'alice.brown@example.com', crypt('user_password', gen_salt('bf')), 'inactive',
 ROW('654 Inactive Dr', 'Inactive Village', 'WA', '98001'),
 '{"last_activity": "2023-01-15", "reason": "vacation"}'),
 
('charlie_davis', 'charlie.davis@example.com', crypt('user_password', gen_salt('bf')), 'suspended',
 ROW('987 Suspended Ln', 'Suspended Town', 'OR', '97001'),
 '{"suspension_reason": "policy_violation", "suspended_date": "2024-01-01"}'),
 
('reporter_user', 'reporter@example.com', crypt('reporter_password', gen_salt('bf')), 'active',
 ROW('111 Report St', 'Report City', 'CO', '80201'),
 '{"role": "reporter", "department": "Analytics", "access_level": "read_only"}'),
 
('app_user', 'app@example.com', crypt('app_password', gen_salt('bf')), 'active',
 ROW('222 App Ave', 'App City', 'NV', '89101'),
 '{"role": "application", "api_access": true, "rate_limit": 1000}');

-- Insert sample products
INSERT INTO products (sku, name, description, category_id, price, cost, stock_quantity, is_active, tags, attributes) VALUES
-- Laptops
('LAP-DELL-001', 'Dell XPS 13', 'High-performance ultrabook with 13-inch display', 3, 1299.99, 900.00, 25, TRUE, 
 ARRAY['laptop', 'ultrabook', 'dell', 'xps'], 
 '{"brand": "Dell", "model": "XPS 13", "cpu": "Intel i7", "ram": "16GB", "storage": "512GB SSD", "screen": "13.3 inch"}'),

('LAP-MAC-001', 'MacBook Air M2', 'Apple MacBook Air with M2 chip', 3, 1199.99, 800.00, 15, TRUE,
 ARRAY['laptop', 'macbook', 'apple', 'm2'],
 '{"brand": "Apple", "model": "MacBook Air", "cpu": "M2", "ram": "8GB", "storage": "256GB SSD", "screen": "13.6 inch"}'),

('LAP-HP-001', 'HP Pavilion 15', 'Budget-friendly laptop for everyday use', 3, 799.99, 550.00, 40, TRUE,
 ARRAY['laptop', 'budget', 'hp', 'pavilion'],
 '{"brand": "HP", "model": "Pavilion 15", "cpu": "AMD Ryzen 5", "ram": "8GB", "storage": "256GB SSD", "screen": "15.6 inch"}'),

-- Desktops
('DSK-GAM-001', 'Gaming Desktop Pro', 'High-end gaming desktop computer', 4, 1899.99, 1300.00, 8, TRUE,
 ARRAY['desktop', 'gaming', 'high-end'],
 '{"type": "gaming", "cpu": "Intel i9", "gpu": "RTX 4080", "ram": "32GB", "storage": "1TB NVMe + 2TB HDD"}'),

('DSK-OFF-001', 'Office Desktop Basic', 'Basic desktop for office work', 4, 599.99, 400.00, 30, TRUE,
 ARRAY['desktop', 'office', 'basic'],
 '{"type": "office", "cpu": "Intel i5", "ram": "16GB", "storage": "512GB SSD", "includes": "keyboard, mouse"}'),

-- Smartphones
('PHN-IPH-001', 'iPhone 15 Pro', 'Latest iPhone with Pro features', 6, 999.99, 700.00, 50, TRUE,
 ARRAY['smartphone', 'iphone', 'apple', 'pro'],
 '{"brand": "Apple", "model": "iPhone 15 Pro", "storage": "128GB", "color": "Natural Titanium", "camera": "48MP"}'),

('PHN-SAM-001', 'Samsung Galaxy S24', 'Flagship Android smartphone', 6, 899.99, 600.00, 35, TRUE,
 ARRAY['smartphone', 'samsung', 'galaxy', 'android'],
 '{"brand": "Samsung", "model": "Galaxy S24", "storage": "256GB", "color": "Phantom Black", "camera": "50MP"}'),

-- Audio Equipment
('AUD-SON-001', 'Sony WH-1000XM5', 'Noise-canceling wireless headphones', 9, 399.99, 250.00, 20, TRUE,
 ARRAY['headphones', 'wireless', 'noise-canceling', 'sony'],
 '{"brand": "Sony", "model": "WH-1000XM5", "type": "over-ear", "battery": "30 hours", "features": ["ANC", "Bluetooth"]}'),

('AUD-APP-001', 'AirPods Pro 2', 'Apple wireless earbuds with ANC', 9, 249.99, 150.00, 45, TRUE,
 ARRAY['earbuds', 'wireless', 'apple', 'anc'],
 '{"brand": "Apple", "model": "AirPods Pro 2", "type": "in-ear", "battery": "6 hours + case", "features": ["ANC", "Spatial Audio"]}'),

-- Books
('BK-PROG-001', 'Clean Code', 'A Handbook of Agile Software Craftsmanship', 61, 45.99, 25.00, 100, TRUE,
 ARRAY['programming', 'software', 'clean-code'],
 '{"author": "Robert C. Martin", "pages": 464, "publisher": "Prentice Hall", "isbn": "978-0132350884"}'),

('BK-FICT-001', 'The Great Gatsby', 'Classic American novel', 19, 12.99, 6.00, 200, TRUE,
 ARRAY['fiction', 'classic', 'american'],
 '{"author": "F. Scott Fitzgerald", "pages": 180, "publisher": "Various", "genre": "Classic Fiction"}'),

-- Clothing
('CLO-TSH-001', 'Cotton T-Shirt', 'Comfortable cotton t-shirt', 12, 24.99, 12.00, 150, TRUE,
 ARRAY['clothing', 'tshirt', 'cotton', 'casual'],
 '{"material": "100% Cotton", "sizes": ["S", "M", "L", "XL"], "colors": ["White", "Black", "Navy"]}'),

('CLO-JNS-001', 'Denim Jeans', 'Classic blue denim jeans', 12, 79.99, 40.00, 80, TRUE,
 ARRAY['clothing', 'jeans', 'denim'],
 '{"material": "98% Cotton, 2% Elastane", "fit": "Regular", "colors": ["Blue", "Black", "Dark Blue"]}'),

-- Out of stock items for testing
('ELC-TAB-001', 'iPad Air', 'Apple iPad Air tablet', 7, 599.99, 400.00, 0, TRUE,
 ARRAY['tablet', 'ipad', 'apple'],
 '{"brand": "Apple", "model": "iPad Air", "storage": "64GB", "screen": "10.9 inch", "connectivity": "Wi-Fi"}'),

-- Inactive product for testing
('OLD-PRD-001', 'Discontinued Item', 'Old product no longer sold', 1, 99.99, 50.00, 5, FALSE,
 ARRAY['discontinued', 'old'],
 '{"status": "discontinued", "replacement": "Contact support"}');

-- Insert sample orders
INSERT INTO orders (user_id, status, total_amount, tax_amount, shipping_cost, payment_method, order_date) VALUES
-- Recent orders
(2, 'delivered', 1349.98, 108.00, 15.99, 'credit_card', '2024-03-15 10:30:00'::timestamp),
(3, 'shipped', 899.99, 72.00, 12.99, 'debit_card', '2024-03-20 14:15:00'::timestamp),
(2, 'processing', 649.98, 52.00, 9.99, 'credit_card', '2024-03-22 09:45:00'::timestamp),
(4, 'pending', 1199.99, 96.00, 19.99, 'paypal', '2024-03-25 16:20:00'::timestamp),

-- Older orders for analytics
(2, 'delivered', 45.99, 3.68, 5.99, 'credit_card', '2024-01-10 11:00:00'::timestamp),
(3, 'delivered', 1299.99, 104.00, 15.99, 'credit_card', '2024-01-15 13:30:00'::timestamp),
(5, 'delivered', 24.99, 2.00, 4.99, 'debit_card', '2024-02-01 10:15:00'::timestamp),
(2, 'delivered', 399.99, 32.00, 7.99, 'credit_card', '2024-02-10 15:45:00'::timestamp),

-- Cancelled order
(6, 'cancelled', 799.99, 64.00, 12.99, 'credit_card', '2024-02-20 12:00:00'::timestamp);

-- Insert order items
INSERT INTO order_items (order_id, product_id, quantity, unit_price, discount_percent) VALUES
-- Order 1: Dell laptop + accessories
(1, 1, 1, 1299.99, 0), -- Dell XPS 13
(1, 9, 1, 49.99, 0),   -- Some accessory (using available product)

-- Order 2: Samsung phone
(2, 7, 1, 899.99, 0), -- Samsung Galaxy S24

-- Order 3: Books and accessories
(3, 10, 1, 45.99, 0),  -- Clean Code book
(3, 11, 2, 12.99, 10), -- The Great Gatsby (2 copies, 10% discount)
(3, 12, 2, 24.99, 0),  -- Cotton T-Shirt

-- Order 4: MacBook
(4, 2, 1, 1199.99, 0), -- MacBook Air M2

-- Order 5: Book
(5, 10, 1, 45.99, 0), -- Clean Code

-- Order 6: Dell laptop
(6, 1, 1, 1299.99, 0), -- Dell XPS 13

-- Order 7: T-shirt
(7, 12, 1, 24.99, 0), -- Cotton T-Shirt

-- Order 8: Headphones
(8, 8, 1, 399.99, 0), -- Sony headphones

-- Order 9: HP laptop (cancelled)
(9, 3, 1, 799.99, 0); -- HP Pavilion 15

-- Re-enable triggers
SET session_replication_role = DEFAULT;

-- Update some users' login information
UPDATE users SET 
    last_login = CURRENT_TIMESTAMP - INTERVAL '1 day',
    login_count = 5
WHERE username IN ('john_doe', 'jane_smith');

UPDATE users SET 
    last_login = CURRENT_TIMESTAMP - INTERVAL '30 days',
    login_count = 15
WHERE username = 'alice_brown';

UPDATE users SET 
    last_login = CURRENT_TIMESTAMP - INTERVAL '2 hours',
    login_count = 50
WHERE username = 'admin_user';

-- Refresh materialized views with new data
REFRESH MATERIALIZED VIEW reporting_schema.category_stats;
REFRESH MATERIALIZED VIEW reporting_schema.inventory_status;

-- Insert some manual audit log entries for testing
INSERT INTO audit_log (table_name, operation, record_id, new_values) VALUES
('products', 'STOCK_UPDATE', 1, '{"stock_change": -5, "reason": "sale", "previous_stock": 30}'),
('products', 'PRICE_UPDATE', 2, '{"old_price": 1149.99, "new_price": 1199.99, "reason": "inflation_adjustment"}'),
('users', 'LOGIN', 2, '{"login_time": "2024-03-25 08:30:00", "ip_address": "192.168.1.100"}'),
('orders', 'STATUS_CHANGE', 1, '{"old_status": "processing", "new_status": "shipped", "tracking_number": "1Z999AA1234567890"}');

-- Create some test data in the main testdb database for cross-database testing
\c testdb;

-- Create a simple test table in the default database
CREATE TABLE IF NOT EXISTS simple_test (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert test data
INSERT INTO simple_test (name) VALUES
('Test Entry 1'),
('Test Entry 2'),
('Test Entry 3');

-- Return to app_db
\c app_db;

-- Add some comments for documentation
COMMENT ON DATABASE app_db IS 'Test database with comprehensive schema for pg-genrole testing';

-- Final summary
DO $$
DECLARE
    user_count INTEGER;
    product_count INTEGER;
    order_count INTEGER;
    category_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO user_count FROM app_schema.users;
    SELECT COUNT(*) INTO product_count FROM app_schema.products;
    SELECT COUNT(*) INTO order_count FROM app_schema.orders;
    SELECT COUNT(*) INTO category_count FROM app_schema.categories;
    
    RAISE NOTICE 'Sample data loaded successfully:';
    RAISE NOTICE '- Users: %', user_count;
    RAISE NOTICE '- Products: %', product_count;
    RAISE NOTICE '- Orders: %', order_count;
    RAISE NOTICE '- Categories: %', category_count;
END $$;