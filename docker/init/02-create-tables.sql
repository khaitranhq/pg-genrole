-- 02-create-tables.sql
-- Create comprehensive table structures for testing all PostgreSQL object types

\c app_db;

SET search_path TO app_schema, public;

-- Enable extensions needed for testing
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create custom domain and types for testing
CREATE DOMAIN email_domain AS TEXT
CHECK (VALUE ~ '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$');

CREATE TYPE user_status AS ENUM ('active', 'inactive', 'pending', 'suspended');
CREATE TYPE address_type AS (
    street VARCHAR(255),
    city VARCHAR(100),
    state VARCHAR(50),
    zipcode VARCHAR(20)
);

-- Create sequences for testing sequence permissions
CREATE SEQUENCE user_id_seq START 1000 INCREMENT 1;
CREATE SEQUENCE order_id_seq START 2000 INCREMENT 1;
CREATE SEQUENCE audit_id_seq START 3000 INCREMENT 1;

-- Main application tables
CREATE TABLE users (
    id INTEGER DEFAULT nextval('user_id_seq') PRIMARY KEY,
    uuid UUID DEFAULT uuid_generate_v4() UNIQUE NOT NULL,
    username VARCHAR(50) UNIQUE NOT NULL,
    email email_domain NOT NULL,
    password_hash TEXT NOT NULL,
    status user_status DEFAULT 'pending',
    address address_type,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP WITH TIME ZONE,
    login_count INTEGER DEFAULT 0,
    metadata JSONB,
    CONSTRAINT users_username_length CHECK (length(username) >= 3)
);

CREATE TABLE categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    parent_id INTEGER REFERENCES categories(id) ON DELETE CASCADE,
    is_active BOOLEAN DEFAULT TRUE,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    sku VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    price DECIMAL(10,2) NOT NULL CHECK (price >= 0),
    cost DECIMAL(10,2) CHECK (cost >= 0),
    stock_quantity INTEGER DEFAULT 0 CHECK (stock_quantity >= 0),
    is_active BOOLEAN DEFAULT TRUE,
    tags TEXT[],
    attributes JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE orders (
    id INTEGER DEFAULT nextval('order_id_seq') PRIMARY KEY,
    uuid UUID DEFAULT uuid_generate_v4() UNIQUE NOT NULL,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'shipped', 'delivered', 'cancelled')),
    total_amount DECIMAL(12,2) NOT NULL CHECK (total_amount >= 0),
    tax_amount DECIMAL(10,2) DEFAULT 0 CHECK (tax_amount >= 0),
    shipping_cost DECIMAL(10,2) DEFAULT 0 CHECK (shipping_cost >= 0),
    discount_amount DECIMAL(10,2) DEFAULT 0 CHECK (discount_amount >= 0),
    payment_method VARCHAR(50),
    shipping_address address_type,
    order_date TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    shipped_date TIMESTAMP WITH TIME ZONE,
    delivered_date TIMESTAMP WITH TIME ZONE,
    notes TEXT
);

CREATE TABLE order_items (
    id SERIAL PRIMARY KEY,
    order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price DECIMAL(10,2) NOT NULL CHECK (unit_price >= 0),
    total_price DECIMAL(12,2) GENERATED ALWAYS AS (quantity * unit_price) STORED,
    discount_percent DECIMAL(5,2) DEFAULT 0 CHECK (discount_percent >= 0 AND discount_percent <= 100),
    UNIQUE(order_id, product_id)
);

-- Audit table for testing trigger permissions
CREATE TABLE audit_log (
    id INTEGER DEFAULT nextval('audit_id_seq') PRIMARY KEY,
    table_name VARCHAR(50) NOT NULL,
    operation VARCHAR(50) NOT NULL,
    record_id INTEGER NOT NULL,
    old_values JSONB,
    new_values JSONB,
    changed_by VARCHAR(50) DEFAULT current_user,
    changed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for testing index permissions
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_users_created_at ON users(created_at);
CREATE INDEX idx_products_category ON products(category_id);
CREATE INDEX idx_products_price ON products(price);
CREATE INDEX idx_products_sku ON products(sku);
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_date ON orders(order_date);
CREATE INDEX idx_order_items_order_id ON order_items(order_id);
CREATE INDEX idx_order_items_product_id ON order_items(product_id);
CREATE INDEX idx_audit_table_name ON audit_log(table_name);
CREATE INDEX idx_audit_changed_at ON audit_log(changed_at);

-- GIN index for JSONB columns
CREATE INDEX idx_users_metadata_gin ON users USING gin(metadata);
CREATE INDEX idx_products_attributes_gin ON products USING gin(attributes);

-- Partial indexes for testing
CREATE INDEX idx_active_products ON products(name) WHERE is_active = TRUE;
-- Partial index for recent orders (using a fixed date for immutability)
-- In production, this would typically use a stored procedure or be managed by the application
CREATE INDEX idx_recent_orders ON orders(order_date) WHERE order_date > '2024-01-01';

-- Comments for documentation
COMMENT ON TABLE users IS 'Application users with authentication and profile data';
COMMENT ON TABLE categories IS 'Product categories with hierarchical structure';
COMMENT ON TABLE products IS 'Product catalog with pricing and inventory';
COMMENT ON TABLE orders IS 'Customer orders with status tracking';
COMMENT ON TABLE order_items IS 'Individual items within orders';
COMMENT ON TABLE audit_log IS 'Audit trail for data changes';

COMMENT ON COLUMN users.uuid IS 'Universal unique identifier for external API references';
COMMENT ON COLUMN users.email IS 'User email with domain validation';
COMMENT ON COLUMN users.status IS 'Account status for access control';
COMMENT ON COLUMN products.sku IS 'Stock Keeping Unit - unique product identifier';
COMMENT ON COLUMN products.attributes IS 'Flexible product attributes in JSON format';
COMMENT ON COLUMN orders.total_amount IS 'Total order amount including tax and shipping';
