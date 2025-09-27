// Package database provides PostgreSQL connection and management functionality
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connection wraps a PostgreSQL connection pool
type Connection struct {
	pool *pgxpool.Pool
	ctx  context.Context
}

// Connect creates a new database connection
func Connect(ctx context.Context, databaseURL string) (*Connection, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Connection{
		pool: pool,
		ctx:  ctx,
	}, nil
}

// Close closes the database connection
func (c *Connection) Close() {
	if c.pool != nil {
		c.pool.Close()
	}
}

// QueryRow executes a query that is expected to return at most one row
func (c *Connection) QueryRow(sql string, args ...any) pgx.Row {
	return c.pool.QueryRow(c.ctx, sql, args...)
}

// Query executes a query that returns rows
func (c *Connection) Query(sql string, args ...any) (pgx.Rows, error) {
	return c.pool.Query(c.ctx, sql, args...)
}

// Exec executes a SQL command
func (c *Connection) Exec(sql string, args ...any) (pgconn.CommandTag, error) {
	return c.pool.Exec(c.ctx, sql, args...)
}

// Begin starts a database transaction
func (c *Connection) Begin() (pgx.Tx, error) {
	return c.pool.Begin(c.ctx)
}

// GetDatabases returns a list of all databases (excluding system databases)
func (c *Connection) GetDatabases() ([]string, error) {
	query := `
		SELECT datname 
		FROM pg_database 
		WHERE datistemplate = false 
		AND datname NOT IN ('postgres', 'template0', 'template1')
		ORDER BY datname`

	rows, err := c.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query databases: %w", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			return nil, fmt.Errorf("failed to scan database name: %w", err)
		}
		databases = append(databases, dbName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating database rows: %w", err)
	}

	return databases, nil
}

// CheckRoleExists checks if a role exists in the database
func (c *Connection) CheckRoleExists(roleName string) (bool, error) {
	var exists bool
	err := c.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", roleName).
		Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if role exists: %w", err)
	}
	return exists, nil
}

// GetSchemas returns a list of schemas in the database (excluding system schemas)
func (c *Connection) GetSchemas() ([]string, error) {
	query := `
		SELECT schema_name 
		FROM information_schema.schemata 
		WHERE schema_name NOT IN ('information_schema', 'pg_catalog', 'pg_toast')
		AND schema_name NOT LIKE 'pg_temp_%'
		AND schema_name NOT LIKE 'pg_toast_temp_%'
		ORDER BY schema_name`

	rows, err := c.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query schemas: %w", err)
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var schemaName string
		if err := rows.Scan(&schemaName); err != nil {
			return nil, fmt.Errorf("failed to scan schema name: %w", err)
		}
		schemas = append(schemas, schemaName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating schema rows: %w", err)
	}

	return schemas, nil
}
