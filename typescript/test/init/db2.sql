-- Create database
CREATE DATABASE db2;

-- Now set up test database with the same schema
\c db2;

-- Create schemas
CREATE SCHEMA IF NOT EXISTS app;
CREATE SCHEMA IF NOT EXISTS audit;

-- Create sequences
CREATE SEQUENCE app.user_id_seq
    INCREMENT 1
    START 1000
    MINVALUE 1000
    MAXVALUE 9999999999;

-- Create tables
CREATE TABLE app.users (
    id INTEGER PRIMARY KEY DEFAULT nextval('app.user_id_seq'),
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE app.posts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES app.users(id),
    title VARCHAR(200) NOT NULL,
    content TEXT,
    status VARCHAR(20) DEFAULT 'draft',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create audit table
CREATE TABLE audit.user_changes (
    id SERIAL PRIMARY KEY,
    user_id INTEGER,
    action VARCHAR(20),
    changed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    old_data JSONB,
    new_data JSONB
);

-- Create function for updating timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger for users table
CREATE TRIGGER update_users_modtime
    BEFORE UPDATE ON app.users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Create function for audit logging
CREATE OR REPLACE FUNCTION audit.log_user_changes()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        INSERT INTO audit.user_changes (user_id, action, old_data, new_data)
        VALUES (OLD.id, 'UPDATE', 
            to_jsonb(OLD), 
            to_jsonb(NEW));
    ELSIF TG_OP = 'DELETE' THEN
        INSERT INTO audit.user_changes (user_id, action, old_data)
        VALUES (OLD.id, 'DELETE', to_jsonb(OLD));
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for audit logging
CREATE TRIGGER users_audit_trigger
    AFTER UPDATE OR DELETE ON app.users
    FOR EACH ROW
    EXECUTE FUNCTION audit.log_user_changes();

-- Create view for active users
CREATE VIEW app.active_users AS
SELECT u.id, u.username, u.email, COUNT(p.id) as post_count
FROM app.users u
LEFT JOIN app.posts p ON u.id = p.user_id
GROUP BY u.id, u.username, u.email;

-- Create materialized view for post statistics
CREATE MATERIALIZED VIEW app.post_stats AS
SELECT 
    date_trunc('day', created_at) as post_date,
    status,
    COUNT(*) as post_count,
    COUNT(DISTINCT user_id) as unique_authors
FROM app.posts
GROUP BY date_trunc('day', created_at), status
WITH DATA;

-- Create index on materialized view
CREATE INDEX idx_post_stats_date ON app.post_stats(post_date);

-- Create function to refresh materialized view
CREATE OR REPLACE FUNCTION app.refresh_post_stats()
RETURNS void AS $$
BEGIN
    REFRESH MATERIALIZED VIEW CONCURRENTLY app.post_stats;
END;
$$ LANGUAGE plpgsql;

-- Insert test data
INSERT INTO app.users (email, username)
VALUES 
    ('test1@example.com', 'test_user1'),
    ('test2@example.com', 'test_user2');

INSERT INTO app.posts (user_id, title, content, status)
SELECT 
    u.id,
    'Test Post ' || seq,
    'This is test content for post ' || seq,
    CASE WHEN seq % 2 = 0 THEN 'published' ELSE 'draft' END
FROM app.users u
CROSS JOIN generate_series(1,2) AS seq
WHERE u.username = 'test_user1';

\echo 'Database setup complete!'
