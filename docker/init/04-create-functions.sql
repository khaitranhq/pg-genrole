-- 04-create-functions.sql
-- Create functions and procedures for testing EXECUTE permissions

\c app_db;

SET search_path TO app_schema, public;

-- Utility functions that RO, RW, and Admin users should be able to execute

-- Function to check if email is valid (used in constraints)
CREATE OR REPLACE FUNCTION is_valid_email(email TEXT)
RETURNS BOOLEAN
LANGUAGE plpgsql
IMMUTABLE
AS $$
BEGIN
    RETURN email ~ '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$';
END;
$$;

-- Function to generate SKU
CREATE OR REPLACE FUNCTION generate_sku(category_name TEXT, product_name TEXT)
RETURNS TEXT
LANGUAGE plpgsql
IMMUTABLE
AS $$
BEGIN
    RETURN upper(
        left(regexp_replace(category_name, '[^A-Za-z0-9]', '', 'g'), 3) || 
        '-' || 
        left(regexp_replace(product_name, '[^A-Za-z0-9]', '', 'g'), 6) || 
        '-' || 
        to_char(extract(epoch from now())::integer, 'FM000000')
    );
END;
$$;

-- Function to calculate order total (read-only calculation)
CREATE OR REPLACE FUNCTION calculate_order_total(order_id_param INTEGER)
RETURNS DECIMAL(12,2)
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
    total DECIMAL(12,2);
BEGIN
    SELECT COALESCE(SUM(quantity * unit_price * (1 - discount_percent / 100)), 0)
    INTO total
    FROM order_items
    WHERE order_id = order_id_param;
    
    RETURN total;
END;
$$;

-- Function to get user's order count (read-only)
CREATE OR REPLACE FUNCTION get_user_order_count(user_id_param INTEGER)
RETURNS INTEGER
LANGUAGE sql
STABLE
AS $$
    SELECT COUNT(*)::integer 
    FROM orders 
    WHERE user_id = user_id_param;
$$;

-- Function to get product stock level (read-only)
CREATE OR REPLACE FUNCTION get_stock_level(product_id_param INTEGER)
RETURNS TEXT
LANGUAGE sql
STABLE
AS $$
    SELECT CASE 
        WHEN stock_quantity = 0 THEN 'OUT_OF_STOCK'
        WHEN stock_quantity < 10 THEN 'LOW_STOCK'
        WHEN stock_quantity < 50 THEN 'MEDIUM_STOCK'
        ELSE 'HIGH_STOCK'
    END
    FROM products 
    WHERE id = product_id_param;
$$;

-- Functions that modify data (RW and Admin users)

-- Function to update user last login
CREATE OR REPLACE FUNCTION update_user_login(user_id_param INTEGER)
RETURNS BOOLEAN
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE users 
    SET 
        last_login = CURRENT_TIMESTAMP,
        login_count = login_count + 1,
        updated_at = CURRENT_TIMESTAMP
    WHERE id = user_id_param;
    
    RETURN FOUND;
END;
$$;

-- Function to create a new order with items
CREATE OR REPLACE FUNCTION create_order_with_items(
    user_id_param INTEGER,
    items JSONB, -- [{"product_id": 1, "quantity": 2, "unit_price": 99.99}, ...]
    shipping_cost_param DECIMAL(10,2) DEFAULT 0,
    tax_rate_param DECIMAL(5,4) DEFAULT 0.08
)
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    new_order_id INTEGER;
    item JSONB;
    subtotal DECIMAL(12,2) := 0;
    tax_amount DECIMAL(10,2);
    total_amount DECIMAL(12,2);
BEGIN
    -- Create the order
    INSERT INTO orders (user_id, status, shipping_cost)
    VALUES (user_id_param, 'pending', shipping_cost_param)
    RETURNING id INTO new_order_id;
    
    -- Add order items and calculate subtotal
    FOR item IN SELECT * FROM jsonb_array_elements(items)
    LOOP
        INSERT INTO order_items (order_id, product_id, quantity, unit_price)
        VALUES (
            new_order_id,
            (item->>'product_id')::INTEGER,
            (item->>'quantity')::INTEGER,
            (item->>'unit_price')::DECIMAL(10,2)
        );
        
        subtotal := subtotal + (item->>'quantity')::INTEGER * (item->>'unit_price')::DECIMAL(10,2);
    END LOOP;
    
    -- Calculate tax and total
    tax_amount := subtotal * tax_rate_param;
    total_amount := subtotal + tax_amount + shipping_cost_param;
    
    -- Update order with calculated amounts
    UPDATE orders 
    SET 
        total_amount = total_amount,
        tax_amount = tax_amount
    WHERE id = new_order_id;
    
    RETURN new_order_id;
END;
$$;

-- Function to update product stock
CREATE OR REPLACE FUNCTION update_product_stock(
    product_id_param INTEGER,
    quantity_change INTEGER
)
RETURNS BOOLEAN
LANGUAGE plpgsql
AS $$
DECLARE
    current_stock INTEGER;
BEGIN
    -- Get current stock
    SELECT stock_quantity INTO current_stock
    FROM products
    WHERE id = product_id_param;
    
    IF NOT FOUND THEN
        RETURN FALSE;
    END IF;
    
    -- Check if we have enough stock for negative changes
    IF current_stock + quantity_change < 0 THEN
        RAISE EXCEPTION 'Insufficient stock. Current: %, Requested change: %', current_stock, quantity_change;
    END IF;
    
    -- Update stock
    UPDATE products 
    SET 
        stock_quantity = stock_quantity + quantity_change,
        updated_at = CURRENT_TIMESTAMP
    WHERE id = product_id_param;
    
    RETURN TRUE;
END;
$$;

-- Administrative functions (Admin users only)

-- Function to reset user password (admin function)
CREATE OR REPLACE FUNCTION admin_reset_user_password(
    user_id_param INTEGER,
    new_password_hash TEXT
)
RETURNS BOOLEAN
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
BEGIN
    UPDATE users 
    SET 
        password_hash = new_password_hash,
        updated_at = CURRENT_TIMESTAMP
    WHERE id = user_id_param;
    
    -- Log the password reset
    INSERT INTO audit_log (table_name, operation, record_id, new_values)
    VALUES ('users', 'UPDATE', user_id_param, jsonb_build_object('password_reset', true, 'reset_by', current_user));
    
    RETURN FOUND;
END;
$$;

-- Function to bulk update product prices (admin function)
CREATE OR REPLACE FUNCTION admin_bulk_price_update(
    category_id_param INTEGER,
    price_multiplier DECIMAL(5,3)
)
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    updated_count INTEGER := 0;
BEGIN
    UPDATE products 
    SET 
        price = price * price_multiplier,
        updated_at = CURRENT_TIMESTAMP
    WHERE category_id = category_id_param;
    
    GET DIAGNOSTICS updated_count = ROW_COUNT;
    
    -- Log the bulk update
    INSERT INTO audit_log (table_name, operation, record_id, new_values)
    VALUES ('products', 'UPDATE', category_id_param, 
            jsonb_build_object('bulk_price_update', true, 'multiplier', price_multiplier, 'affected_count', updated_count));
    
    RETURN updated_count;
END;
$$;

-- Stored procedures for testing

-- Procedure to process order fulfillment
CREATE OR REPLACE PROCEDURE process_order_fulfillment(order_id_param INTEGER)
LANGUAGE plpgsql
AS $$
DECLARE
    order_status_var VARCHAR(20);
    item RECORD;
BEGIN
    -- Check order status
    SELECT status INTO order_status_var
    FROM orders
    WHERE id = order_id_param;
    
    IF order_status_var != 'pending' THEN
        RAISE EXCEPTION 'Order % is not in pending status. Current status: %', order_id_param, order_status_var;
    END IF;
    
    -- Update stock quantities for all items
    FOR item IN 
        SELECT product_id, quantity 
        FROM order_items 
        WHERE order_id = order_id_param
    LOOP
        PERFORM update_product_stock(item.product_id, -item.quantity);
    END LOOP;
    
    -- Update order status
    UPDATE orders 
    SET status = 'processing' 
    WHERE id = order_id_param;
    
    -- Log the fulfillment
    INSERT INTO audit_log (table_name, operation, record_id, new_values)
    VALUES ('orders', 'UPDATE', order_id_param, jsonb_build_object('status_change', 'pending->processing'));
    
    COMMIT;
END;
$$;

-- Procedure to clean old audit logs (admin procedure)
CREATE OR REPLACE PROCEDURE admin_cleanup_audit_logs(days_to_keep INTEGER DEFAULT 90)
LANGUAGE plpgsql
AS $$
DECLARE
    deleted_count INTEGER := 0;
BEGIN
    DELETE FROM audit_log 
    WHERE changed_at < CURRENT_TIMESTAMP - (days_to_keep || ' days')::INTERVAL;
    
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    
    RAISE NOTICE 'Cleaned up % old audit log records older than % days', deleted_count, days_to_keep;
END;
$$;

-- Comments on functions and procedures
COMMENT ON FUNCTION is_valid_email(TEXT) IS 'Validates email format using regex pattern';
COMMENT ON FUNCTION generate_sku(TEXT, TEXT) IS 'Generates unique SKU from category and product name';
COMMENT ON FUNCTION calculate_order_total(INTEGER) IS 'Calculates total amount for an order including discounts';
COMMENT ON FUNCTION get_user_order_count(INTEGER) IS 'Returns total number of orders for a user';
COMMENT ON FUNCTION get_stock_level(INTEGER) IS 'Returns stock level category for a product';
COMMENT ON FUNCTION update_user_login(INTEGER) IS 'Updates user login timestamp and count';
COMMENT ON FUNCTION create_order_with_items(INTEGER, JSONB, DECIMAL, DECIMAL) IS 'Creates order with items and calculates totals';
COMMENT ON FUNCTION update_product_stock(INTEGER, INTEGER) IS 'Updates product stock quantity with validation';
COMMENT ON FUNCTION admin_reset_user_password(INTEGER, TEXT) IS 'Admin function to reset user password with audit trail';
COMMENT ON FUNCTION admin_bulk_price_update(INTEGER, DECIMAL) IS 'Admin function for bulk price updates by category';
COMMENT ON PROCEDURE process_order_fulfillment(INTEGER) IS 'Processes order fulfillment and updates stock';
COMMENT ON PROCEDURE admin_cleanup_audit_logs(INTEGER) IS 'Admin procedure to clean up old audit log entries';