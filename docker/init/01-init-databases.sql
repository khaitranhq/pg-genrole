-- 01-init-databases.sql
-- Initialize databases and base configuration for pg-genrole testing

-- Create additional test databases
CREATE DATABASE app_db;
CREATE DATABASE reporting_db;
CREATE DATABASE analytics_db;

-- Create application user for testing connection scenarios
CREATE USER app_admin WITH PASSWORD 'app_admin_pass';

-- Grant database creation privilege to app_admin for testing
ALTER USER app_admin CREATEDB;

-- Connect to app_db for further setup
\c app_db;

-- Create schemas to test schema-level permissions
CREATE SCHEMA IF NOT EXISTS public;
CREATE SCHEMA IF NOT EXISTS app_schema;
CREATE SCHEMA IF NOT EXISTS reporting_schema;
CREATE SCHEMA IF NOT EXISTS admin_schema;

-- Grant schema usage to app_admin for setup
GRANT USAGE ON SCHEMA public TO app_admin;
GRANT USAGE ON SCHEMA app_schema TO app_admin;
GRANT USAGE ON SCHEMA reporting_schema TO app_admin;
GRANT USAGE ON SCHEMA admin_schema TO app_admin;

-- Set search path
ALTER DATABASE app_db SET search_path TO app_schema, public;

COMMENT ON DATABASE app_db IS 'Primary application database for pg-genrole testing';
COMMENT ON SCHEMA app_schema IS 'Main application schema';
COMMENT ON SCHEMA reporting_schema IS 'Reporting and analytics schema';
COMMENT ON SCHEMA admin_schema IS 'Administrative operations schema';