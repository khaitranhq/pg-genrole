-- Bootstrap a dedicated database, then seed schema + sample data into it.
-- docker-entrypoint-initdb.d runs this file with psql, so the CREATE DATABASE
-- and \c (connect) commands take effect there. The e2e tests create their own
-- databases and execute only the schema part (everything after the \c line).
-- User emails/usernames embed the database name via current_database().

CREATE DATABASE app;

\c app

CREATE SCHEMA IF NOT EXISTS app;
CREATE SCHEMA IF NOT EXISTS audit;

CREATE SEQUENCE app.user_id_seq INCREMENT 1 START 1000 MINVALUE 1000 MAXVALUE 9999999999;
CREATE SEQUENCE app.post_id_seq INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 9999999999;

CREATE TABLE app.users (
	id INTEGER PRIMARY KEY DEFAULT nextval('app.user_id_seq'),
	email VARCHAR(255) UNIQUE NOT NULL,
	username VARCHAR(50) NOT NULL,
	role VARCHAR(20) DEFAULT 'user',
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE app.posts (
	id INTEGER PRIMARY KEY DEFAULT nextval('app.post_id_seq'),
	user_id INTEGER REFERENCES app.users(id),
	title VARCHAR(200) NOT NULL,
	content TEXT,
	status VARCHAR(20) DEFAULT 'draft',
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE app.comments (
	id SERIAL PRIMARY KEY,
	post_id INTEGER REFERENCES app.posts(id),
	user_id INTEGER REFERENCES app.users(id),
	content TEXT NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE audit.changes (
	id SERIAL PRIMARY KEY,
	table_name VARCHAR(50),
	record_id INTEGER,
	action VARCHAR(20),
	changed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	old_data JSONB,
	new_data JSONB
);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
	NEW.updated_at = CURRENT_TIMESTAMP;
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION audit.log_changes()
RETURNS TRIGGER AS $$
BEGIN
	IF TG_OP = 'UPDATE' THEN
		INSERT INTO audit.changes (table_name, record_id, action, old_data, new_data)
		VALUES (TG_TABLE_NAME, OLD.id, 'UPDATE', to_jsonb(OLD), to_jsonb(NEW));
	ELSIF TG_OP = 'DELETE' THEN
		INSERT INTO audit.changes (table_name, record_id, action, old_data)
		VALUES (TG_TABLE_NAME, OLD.id, 'DELETE', to_jsonb(OLD));
	END IF;
	RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION app.get_user_stats(p_user_id INTEGER)
RETURNS TABLE (post_count INTEGER, comment_count INTEGER, last_activity TIMESTAMP WITH TIME ZONE) AS $$
BEGIN
	RETURN QUERY
	SELECT
		COUNT(DISTINCT p.id)::INTEGER,
		COUNT(DISTINCT c.id)::INTEGER,
		MAX(GREATEST(COALESCE(p.created_at, '1970-01-01'), COALESCE(c.created_at, '1970-01-01')))
	FROM app.users u
	LEFT JOIN app.posts p ON p.user_id = u.id
	LEFT JOIN app.comments c ON c.user_id = u.id
	WHERE u.id = p_user_id;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_users_modtime BEFORE UPDATE ON app.users
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_posts_modtime BEFORE UPDATE ON app.posts
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER audit_users_changes AFTER UPDATE OR DELETE ON app.users
FOR EACH ROW EXECUTE FUNCTION audit.log_changes();
CREATE TRIGGER audit_posts_changes AFTER UPDATE OR DELETE ON app.posts
FOR EACH ROW EXECUTE FUNCTION audit.log_changes();

CREATE VIEW app.active_users AS
SELECT u.id, u.username, u.email,
	COUNT(DISTINCT p.id) as post_count,
	COUNT(DISTINCT c.id) as comment_count,
	MAX(GREATEST(p.created_at, c.created_at)) as last_activity
FROM app.users u
LEFT JOIN app.posts p ON p.user_id = u.id
LEFT JOIN app.comments c ON c.user_id = u.id
GROUP BY u.id, u.username, u.email;

CREATE MATERIALIZED VIEW app.post_stats AS
SELECT date_trunc('day', created_at) as post_date, status,
	COUNT(*) as post_count, COUNT(DISTINCT user_id) as unique_authors
FROM app.posts
GROUP BY date_trunc('day', created_at), status
WITH DATA;

CREATE INDEX idx_posts_user_id ON app.posts(user_id);
CREATE INDEX idx_comments_post_id ON app.comments(post_id);
CREATE INDEX idx_comments_user_id ON app.comments(user_id);
CREATE INDEX idx_post_stats_date ON app.post_stats(post_date);

INSERT INTO app.users (email, username, role)
SELECT 'user1@' || current_database() || '.com', 'user1_' || current_database(), 'admin'
UNION ALL
SELECT 'user2@' || current_database() || '.com', 'user2_' || current_database(), 'user'
UNION ALL
SELECT 'user3@' || current_database() || '.com', 'user3_' || current_database(), 'user';

INSERT INTO app.posts (user_id, title, content, status)
SELECT u.id, 'Post ' || generate_series || ' by ' || u.username,
	'Content for post ' || generate_series || ' in database',
	CASE WHEN generate_series % 2 = 0 THEN 'published' ELSE 'draft' END
FROM app.users u CROSS JOIN generate_series(1, 3);

INSERT INTO app.comments (post_id, user_id, content)
SELECT p.id, u.id, 'Comment on post ' || p.id || ' by ' || u.username
FROM app.posts p CROSS JOIN app.users u
WHERE p.status = 'published';

REFRESH MATERIALIZED VIEW app.post_stats;
