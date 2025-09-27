-- 03-create-views.sql
-- Create views and materialized views for testing view permissions

\c app_db;

SET search_path TO app_schema, public;

-- Regular views for testing SELECT permissions
CREATE VIEW user_summary AS
SELECT 
    id,
    username,
    email,
    status,
    created_at,
    last_login,
    login_count,
    CASE 
        WHEN last_login > CURRENT_TIMESTAMP - INTERVAL '30 days' THEN 'Active'
        WHEN last_login IS NULL THEN 'Never Logged In'
        ELSE 'Inactive'
    END as activity_status
FROM users;

CREATE VIEW active_products AS
SELECT 
    p.id,
    p.sku,
    p.name,
    p.description,
    c.name as category_name,
    p.price,
    p.stock_quantity,
    p.created_at,
    p.updated_at
FROM products p
LEFT JOIN categories c ON p.category_id = c.id
WHERE p.is_active = TRUE;

CREATE VIEW order_summary AS
SELECT 
    o.id,
    o.uuid,
    u.username,
    u.email,
    o.status,
    o.total_amount,
    o.order_date,
    o.shipped_date,
    o.delivered_date,
    COUNT(oi.id) as item_count,
    SUM(oi.quantity) as total_items
FROM orders o
JOIN users u ON o.user_id = u.id
LEFT JOIN order_items oi ON o.id = oi.order_id
GROUP BY o.id, o.uuid, u.username, u.email, o.status, o.total_amount, o.order_date, o.shipped_date, o.delivered_date;

-- Reporting views for the reporting schema
SET search_path TO reporting_schema, app_schema, public;

CREATE VIEW daily_sales AS
SELECT 
    DATE(order_date) as sale_date,
    COUNT(*) as order_count,
    SUM(total_amount) as total_revenue,
    AVG(total_amount) as average_order_value,
    COUNT(DISTINCT user_id) as unique_customers
FROM app_schema.orders 
WHERE status IN ('shipped', 'delivered')
GROUP BY DATE(order_date)
ORDER BY sale_date DESC;

CREATE VIEW monthly_sales AS
SELECT 
    DATE_TRUNC('month', order_date) as month,
    COUNT(*) as order_count,
    SUM(total_amount) as total_revenue,
    AVG(total_amount) as average_order_value,
    COUNT(DISTINCT user_id) as unique_customers
FROM app_schema.orders 
WHERE status IN ('shipped', 'delivered')
GROUP BY DATE_TRUNC('month', order_date)
ORDER BY month DESC;

CREATE VIEW product_performance AS
SELECT 
    p.id,
    p.sku,
    p.name,
    c.name as category_name,
    COUNT(oi.id) as times_ordered,
    SUM(oi.quantity) as total_quantity_sold,
    SUM(oi.total_price) as total_revenue,
    AVG(oi.unit_price) as average_selling_price,
    p.price as current_price,
    ROUND((SUM(oi.total_price) / NULLIF(SUM(oi.quantity), 0))::numeric, 2) as actual_avg_price
FROM app_schema.products p
LEFT JOIN app_schema.categories c ON p.category_id = c.id
LEFT JOIN app_schema.order_items oi ON p.id = oi.product_id
LEFT JOIN app_schema.orders o ON oi.order_id = o.id AND o.status IN ('shipped', 'delivered')
GROUP BY p.id, p.sku, p.name, c.name, p.price
ORDER BY total_revenue DESC NULLS LAST;

-- Customer analysis view
CREATE VIEW customer_analytics AS
SELECT 
    u.id,
    u.username,
    u.email,
    u.status,
    u.created_at as registration_date,
    COUNT(o.id) as total_orders,
    COALESCE(SUM(o.total_amount), 0) as lifetime_value,
    COALESCE(AVG(o.total_amount), 0) as average_order_value,
    MAX(o.order_date) as last_order_date,
    MIN(o.order_date) as first_order_date,
    CASE 
        WHEN COUNT(o.id) = 0 THEN 'No Orders'
        WHEN MAX(o.order_date) > CURRENT_TIMESTAMP - INTERVAL '30 days' THEN 'Active'
        WHEN MAX(o.order_date) > CURRENT_TIMESTAMP - INTERVAL '90 days' THEN 'At Risk'
        ELSE 'Inactive'
    END as customer_segment
FROM app_schema.users u
LEFT JOIN app_schema.orders o ON u.id = o.user_id
GROUP BY u.id, u.username, u.email, u.status, u.created_at;

-- Materialized views for testing REFRESH permissions
CREATE MATERIALIZED VIEW category_stats AS
SELECT 
    c.id,
    c.name,
    c.description,
    COUNT(p.id) as product_count,
    COALESCE(AVG(p.price), 0) as average_price,
    COALESCE(MIN(p.price), 0) as min_price,
    COALESCE(MAX(p.price), 0) as max_price,
    COALESCE(SUM(p.stock_quantity), 0) as total_stock,
    COUNT(p.id) FILTER (WHERE p.is_active = TRUE) as active_product_count
FROM app_schema.categories c
LEFT JOIN app_schema.products p ON c.id = p.category_id
GROUP BY c.id, c.name, c.description
ORDER BY product_count DESC;

CREATE MATERIALIZED VIEW inventory_status AS
SELECT 
    p.id,
    p.sku,
    p.name,
    p.stock_quantity,
    CASE 
        WHEN p.stock_quantity = 0 THEN 'Out of Stock'
        WHEN p.stock_quantity < 10 THEN 'Low Stock'
        WHEN p.stock_quantity < 50 THEN 'Medium Stock'
        ELSE 'High Stock'
    END as stock_status,
    p.price,
    p.cost,
    CASE 
        WHEN p.cost > 0 THEN ROUND(((p.price - p.cost) / p.cost * 100)::numeric, 2)
        ELSE NULL
    END as markup_percentage,
    p.updated_at as last_inventory_update
FROM app_schema.products p
WHERE p.is_active = TRUE
ORDER BY 
    CASE 
        WHEN p.stock_quantity = 0 THEN 1
        WHEN p.stock_quantity < 10 THEN 2
        WHEN p.stock_quantity < 50 THEN 3
        ELSE 4
    END,
    p.stock_quantity ASC;

-- Create unique indexes on materialized views
CREATE UNIQUE INDEX idx_category_stats_id ON category_stats(id);
CREATE UNIQUE INDEX idx_inventory_status_id ON inventory_status(id);

-- Initial refresh of materialized views
REFRESH MATERIALIZED VIEW category_stats;
REFRESH MATERIALIZED VIEW inventory_status;

-- Comments on views
COMMENT ON VIEW user_summary IS 'Simplified user information with activity status';
COMMENT ON VIEW active_products IS 'Currently active products with category information';
COMMENT ON VIEW order_summary IS 'Order information with customer details and item counts';
COMMENT ON VIEW daily_sales IS 'Daily sales metrics for reporting';
COMMENT ON VIEW monthly_sales IS 'Monthly sales metrics for reporting';
COMMENT ON VIEW product_performance IS 'Product sales performance analytics';
COMMENT ON VIEW customer_analytics IS 'Customer behavior and segmentation analysis';
COMMENT ON MATERIALIZED VIEW category_stats IS 'Category statistics with product counts and pricing';
COMMENT ON MATERIALIZED VIEW inventory_status IS 'Current inventory status with stock levels and markup';