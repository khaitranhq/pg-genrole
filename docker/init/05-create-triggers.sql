-- 05-create-triggers.sql
-- Create triggers for testing TRIGGER permissions

\c app_db;

SET search_path TO app_schema, public;

-- Trigger functions for audit logging
CREATE OR REPLACE FUNCTION audit_trigger_function()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    old_data JSONB;
    new_data JSONB;
BEGIN
    -- Handle different trigger operations
    IF TG_OP = 'DELETE' THEN
        old_data := to_jsonb(OLD);
        INSERT INTO audit_log (table_name, operation, record_id, old_values)
        VALUES (TG_TABLE_NAME, TG_OP, OLD.id, old_data);
        RETURN OLD;
    ELSIF TG_OP = 'UPDATE' THEN
        old_data := to_jsonb(OLD);
        new_data := to_jsonb(NEW);
        INSERT INTO audit_log (table_name, operation, record_id, old_values, new_values)
        VALUES (TG_TABLE_NAME, TG_OP, NEW.id, old_data, new_data);
        RETURN NEW;
    ELSIF TG_OP = 'INSERT' THEN
        new_data := to_jsonb(NEW);
        INSERT INTO audit_log (table_name, operation, record_id, new_values)
        VALUES (TG_TABLE_NAME, TG_OP, NEW.id, new_data);
        RETURN NEW;
    END IF;
    RETURN NULL;
END;
$$;

-- Trigger function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

-- Trigger function to validate product stock changes
CREATE OR REPLACE FUNCTION validate_stock_changes()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    -- Prevent negative stock quantities
    IF NEW.stock_quantity < 0 THEN
        RAISE EXCEPTION 'Stock quantity cannot be negative. Attempted value: %', NEW.stock_quantity;
    END IF;
    
    -- Log low stock warnings
    IF NEW.stock_quantity <= 5 AND (OLD.stock_quantity IS NULL OR OLD.stock_quantity > 5) THEN
        INSERT INTO audit_log (table_name, operation, record_id, new_values)
        VALUES ('products', 'WARNING', NEW.id, 
                jsonb_build_object('warning_type', 'low_stock', 'stock_level', NEW.stock_quantity));
    END IF;
    
    RETURN NEW;
END;
$$;

-- Trigger function to validate order totals
CREATE OR REPLACE FUNCTION validate_order_totals()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    calculated_total DECIMAL(12,2);
BEGIN
    -- Calculate expected total from order items
    SELECT COALESCE(SUM(quantity * unit_price * (1 - COALESCE(discount_percent, 0) / 100)), 0)
    INTO calculated_total
    FROM order_items
    WHERE order_id = NEW.id;
    
    -- Add tax and shipping
    calculated_total := calculated_total + COALESCE(NEW.tax_amount, 0) + COALESCE(NEW.shipping_cost, 0);
    
    -- Allow small rounding differences
    IF ABS(calculated_total - NEW.total_amount) > 0.01 THEN
        RAISE EXCEPTION 'Order total mismatch. Calculated: %, Provided: %', calculated_total, NEW.total_amount;
    END IF;
    
    RETURN NEW;
END;
$$;

-- Trigger function to enforce business rules on user status
CREATE OR REPLACE FUNCTION enforce_user_status_rules()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    -- Log status changes
    IF OLD.status IS DISTINCT FROM NEW.status THEN
        INSERT INTO audit_log (table_name, operation, record_id, old_values, new_values)
        VALUES ('users', 'STATUS_CHANGE', NEW.id, 
                jsonb_build_object('old_status', OLD.status),
                jsonb_build_object('new_status', NEW.status));
    END IF;
    
    -- Prevent suspended users from being directly activated
    IF OLD.status = 'suspended' AND NEW.status = 'active' THEN
        RAISE EXCEPTION 'Suspended users must be set to pending before activation';
    END IF;
    
    -- Auto-update last_login when status changes to active
    IF NEW.status = 'active' AND OLD.status != 'active' THEN
        NEW.last_login := CURRENT_TIMESTAMP;
    END IF;
    
    RETURN NEW;
END;
$$;

-- Trigger function to cascade category changes
CREATE OR REPLACE FUNCTION handle_category_changes()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    -- When a category is disabled, log all affected products
    IF TG_OP = 'UPDATE' AND OLD.is_active = TRUE AND NEW.is_active = FALSE THEN
        INSERT INTO audit_log (table_name, operation, record_id, new_values)
        SELECT 'products', 'CATEGORY_DISABLED', id, 
               jsonb_build_object('category_id', NEW.id, 'category_name', NEW.name)
        FROM products 
        WHERE category_id = NEW.id AND is_active = TRUE;
    END IF;
    
    -- When a category is deleted, verify no active products reference it
    IF TG_OP = 'DELETE' THEN
        IF EXISTS (SELECT 1 FROM products WHERE category_id = OLD.id AND is_active = TRUE) THEN
            RAISE EXCEPTION 'Cannot delete category with active products. Category: %', OLD.name;
        END IF;
        RETURN OLD;
    END IF;
    
    RETURN NEW;
END;
$$;

-- Create triggers on various tables

-- Audit triggers for all major tables
CREATE TRIGGER users_audit_trigger
    AFTER INSERT OR UPDATE OR DELETE ON users
    FOR EACH ROW EXECUTE FUNCTION audit_trigger_function();

CREATE TRIGGER products_audit_trigger
    AFTER INSERT OR UPDATE OR DELETE ON products
    FOR EACH ROW EXECUTE FUNCTION audit_trigger_function();

CREATE TRIGGER orders_audit_trigger
    AFTER INSERT OR UPDATE OR DELETE ON orders
    FOR EACH ROW EXECUTE FUNCTION audit_trigger_function();

CREATE TRIGGER categories_audit_trigger
    AFTER INSERT OR UPDATE OR DELETE ON categories
    FOR EACH ROW EXECUTE FUNCTION audit_trigger_function();

-- Updated_at triggers
CREATE TRIGGER users_updated_at_trigger
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER products_updated_at_trigger
    BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Business logic triggers
CREATE TRIGGER products_stock_validation_trigger
    BEFORE INSERT OR UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION validate_stock_changes();

CREATE TRIGGER orders_total_validation_trigger
    BEFORE INSERT OR UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION validate_order_totals();

CREATE TRIGGER users_status_rules_trigger
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION enforce_user_status_rules();

CREATE TRIGGER categories_changes_trigger
    BEFORE UPDATE OR DELETE ON categories
    FOR EACH ROW EXECUTE FUNCTION handle_category_changes();

-- Disable some triggers initially (can be used to test TRIGGER permissions)
ALTER TABLE audit_log DISABLE TRIGGER ALL;

-- Comments on trigger functions
COMMENT ON FUNCTION audit_trigger_function() IS 'Generic audit trigger function for logging data changes';
COMMENT ON FUNCTION update_updated_at_column() IS 'Updates the updated_at timestamp on row changes';
COMMENT ON FUNCTION validate_stock_changes() IS 'Validates stock quantity changes and logs warnings';
COMMENT ON FUNCTION validate_order_totals() IS 'Validates order total calculations against line items';
COMMENT ON FUNCTION enforce_user_status_rules() IS 'Enforces business rules for user status changes';
COMMENT ON FUNCTION handle_category_changes() IS 'Handles cascading effects of category changes';

-- Comments on triggers
COMMENT ON TRIGGER users_audit_trigger ON users IS 'Audits all changes to user records';
COMMENT ON TRIGGER products_audit_trigger ON products IS 'Audits all changes to product records';
COMMENT ON TRIGGER orders_audit_trigger ON orders IS 'Audits all changes to order records';
COMMENT ON TRIGGER categories_audit_trigger ON categories IS 'Audits all changes to category records';
COMMENT ON TRIGGER users_updated_at_trigger ON users IS 'Automatically updates updated_at on user changes';
COMMENT ON TRIGGER products_updated_at_trigger ON products IS 'Automatically updates updated_at on product changes';
COMMENT ON TRIGGER products_stock_validation_trigger ON products IS 'Validates stock levels and logs warnings';
COMMENT ON TRIGGER orders_total_validation_trigger ON orders IS 'Validates order total calculations';
COMMENT ON TRIGGER users_status_rules_trigger ON users IS 'Enforces user status transition rules';
COMMENT ON TRIGGER categories_changes_trigger ON categories IS 'Handles category change side effects';