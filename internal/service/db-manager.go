package service

import (
	"context"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Rows is the subset of pgx.Rows the manager needs.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// Querier executes SQL against a single database connection.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Close(ctx context.Context) error
}

// Connector opens connections to a given database.
type Connector interface {
	Connect(ctx context.Context, database string) (Querier, error)
}

// Config holds the admin connection settings used to manage roles.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
}

// DatabaseManager creates and grants {database}.read and
// {database}.readwrite roles for PostgreSQL databases.
type DatabaseManager struct {
	connector Connector
	debug     bool
}

// pgxConnector dials PostgreSQL via pgx.
type pgxConnector struct {
	cfg Config
}

// pgxConn adapts *pgx.Conn to the Querier interface, converting
// pgx.Rows to the narrower Rows interface.
type pgxConn struct {
	*pgx.Conn
}

func (q *pgxConn) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	//nolint:sqlclosecheck // rows ownership transfers to the caller
	rows, err := q.Conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query %q: %w", sql, err)
	}

	return rows, nil
}

func (c *pgxConnector) Connect(ctx context.Context, database string) (Querier, error) {
	if c.cfg.Port < 1 || c.cfg.Port > 65535 {
		return nil, fmt.Errorf("invalid port %d", c.cfg.Port)
	}
	cfg, err := pgx.ParseConfig("")
	if err != nil {
		return nil, fmt.Errorf("parse pgx config: %w", err)
	}
	cfg.Host = c.cfg.Host
	cfg.Port = uint16(c.cfg.Port)
	cfg.User = c.cfg.User
	cfg.Password = c.cfg.Password
	cfg.Database = database
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", database, err)
	}

	return &pgxConn{Conn: conn}, nil
}

// NewDatabaseManager returns a manager connecting as cfg.
func NewDatabaseManager(cfg Config, debug bool) *DatabaseManager {
	return &DatabaseManager{connector: &pgxConnector{cfg: cfg}, debug: debug}
}

// RefreshRolesPermissions creates and grants {database}.read and
// {database}.readwrite for each database in databases; when empty,
// all non-system databases are processed.
func (m *DatabaseManager) RefreshRolesPermissions(ctx context.Context, databases []string) error {
	admin, err := m.connector.Connect(ctx, "postgres")
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer func() { _ = admin.Close(ctx) }()

	existing, err := listDatabases(ctx, admin)
	if err != nil {
		return fmt.Errorf("list databases: %w", err)
	}

	targets := databases
	if len(targets) == 0 {
		targets = existing
	}
	for _, database := range targets {
		if !slices.Contains(existing, database) {
			return fmt.Errorf("database %s not found", database)
		}
		m.logf("Grant read permission for database %s", database)
		if err := m.grantPermissions(ctx, database, database+".read", false); err != nil {
			return fmt.Errorf("grant read on %s: %w", database, err)
		}
		m.logf("Grant readwrite permission for database %s", database)
		if err := m.grantPermissions(ctx, database, database+".readwrite", true); err != nil {
			return fmt.Errorf("grant readwrite on %s: %w", database, err)
		}
	}

	return nil
}

// grantPermissions creates role if missing, then grants connect and
// per-schema privileges. readwrite adds write and ownership privileges.
func (m *DatabaseManager) grantPermissions(ctx context.Context, database, role string, readwrite bool) error {
	conn, err := m.connector.Connect(ctx, database)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", database, err)
	}
	defer func() { _ = conn.Close(ctx) }()

	exists, err := roleExists(ctx, conn, role)
	if err != nil {
		return err
	}
	if !exists {
		m.logf("Create role %s", role)
		if err := exec(ctx, conn, fmt.Sprintf("CREATE ROLE %s", ident(role))); err != nil {
			return err
		}
	}

	if err := exec(ctx, conn, fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s", ident(database), ident(role))); err != nil {
		return fmt.Errorf("grant connect on %s: %w", database, err)
	}

	if readwrite {
		if err := grantForeignServers(ctx, conn, role); err != nil {
			return err
		}
	}

	schemas, err := listSchemas(ctx, conn)
	if err != nil {
		return err
	}
	for _, schema := range schemas {
		m.logf("Grant permissions for schema %s to %s", schema, role)
		if err := grantSchema(ctx, conn, schema, role, readwrite); err != nil {
			return err
		}
	}

	return nil
}

// grantSchema grants usage, table, matview, view, sequence and function
// privileges on schema, including default privileges for new objects.
func grantSchema(ctx context.Context, conn Querier, schema, role string, readwrite bool) error {
	tablePrivs := "SELECT"
	if readwrite {
		tablePrivs = "SELECT, INSERT, UPDATE, DELETE, TRUNCATE"
	}
	sq, rq := ident(schema), ident(role)

	if err := exec(ctx, conn, fmt.Sprintf("GRANT USAGE ON SCHEMA %s TO %s", sq, rq)); err != nil {
		return fmt.Errorf("grant usage on %s: %w", schema, err)
	}
	if err := exec(ctx, conn, fmt.Sprintf("GRANT %s ON ALL TABLES IN SCHEMA %s TO %s", tablePrivs, sq, rq)); err != nil {
		return fmt.Errorf("grant tables on %s: %w", schema, err)
	}
	if err := exec(ctx, conn, fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT %s ON TABLES TO %s", sq, tablePrivs, rq)); err != nil {
		return fmt.Errorf("grant default table privileges on %s: %w", schema, err)
	}

	if err := grantSelectObjects(ctx, conn, schema, role, "pg_matviews", "matviewname",
		"ALTER MATERIALIZED VIEW %s OWNER TO %s", readwrite); err != nil {
		return fmt.Errorf("grant matviews on %s: %w", schema, err)
	}
	if err := grantSelectObjects(ctx, conn, schema, role, "pg_catalog.pg_views", "viewname", "", false); err != nil {
		return fmt.Errorf("grant views on %s: %w", schema, err)
	}

	if err := exec(ctx, conn, fmt.Sprintf("GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA %s TO %s", sq, rq)); err != nil {
		return fmt.Errorf("grant sequences on %s: %w", schema, err)
	}
	if err := exec(ctx, conn, fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT USAGE, SELECT ON SEQUENCES TO %s", sq, rq)); err != nil {
		return fmt.Errorf("grant default sequence privileges on %s: %w", schema, err)
	}

	if err := exec(ctx, conn, fmt.Sprintf("GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA %s TO %s", sq, rq)); err != nil {
		return fmt.Errorf("grant functions on %s: %w", schema, err)
	}
	if err := exec(ctx, conn, fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT EXECUTE ON FUNCTIONS TO %s", sq, rq)); err != nil {
		return fmt.Errorf("grant default function privileges on %s: %w", schema, err)
	}

	return nil
}

// grantSelectObjects grants SELECT on every object of catalog within
// schema; ownerStmt, when non-empty, additionally transfers ownership.
func grantSelectObjects(ctx context.Context, conn Querier, schema, role, catalog, nameCol, ownerStmt string, readwrite bool) error {
	rows, err := conn.Query(ctx,
		fmt.Sprintf("SELECT %s FROM %s WHERE schemaname = $1", nameCol, catalog), schema)
	if err != nil {
		return fmt.Errorf("list %s objects: %w", catalog, err)
	}
	defer rows.Close()

	// Drain rows before executing grants: pgx holds the single connection
	// busy while a result set is open, so Exec on the same conn would fail.
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan %s object: %w", catalog, err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate %s objects: %w", catalog, err)
	}

	for _, name := range names {
		obj := fmt.Sprintf("%s.%s", ident(schema), ident(name))
		if err := exec(ctx, conn, fmt.Sprintf("GRANT SELECT ON %s TO %s", obj, ident(role))); err != nil {
			return err
		}
		if readwrite && ownerStmt != "" {
			if err := exec(ctx, conn, fmt.Sprintf(ownerStmt, obj, ident(role))); err != nil {
				return err
			}
		}
	}

	return nil
}

func listDatabases(ctx context.Context, conn Querier) ([]string, error) {
	rows, err := conn.Query(ctx, `SELECT datname FROM pg_database WHERE datname NOT IN ('postgres', 'template0', 'template1', 'rdsadmin')`)
	if err != nil {
		return nil, fmt.Errorf("list databases: %w", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan database name: %w", err)
		}
		databases = append(databases, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate databases: %w", err)
	}

	return databases, nil
}

func listSchemas(ctx context.Context, conn Querier) ([]string, error) {
	rows, err := conn.Query(ctx, `SELECT schema_name FROM information_schema.schemata
		WHERE schema_name NOT LIKE '%pg%' AND schema_name != 'information_schema'`)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan schema name: %w", err)
		}
		schemas = append(schemas, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schemas: %w", err)
	}

	return schemas, nil
}

func roleExists(ctx context.Context, conn Querier, role string) (bool, error) {
	rows, err := conn.Query(ctx, `SELECT 1 FROM pg_roles WHERE rolname = $1`, role)
	if err != nil {
		return false, fmt.Errorf("check role %s: %w", role, err)
	}
	defer rows.Close()

	return rows.Next(), rows.Err()
}

func grantForeignServers(ctx context.Context, conn Querier, role string) error {
	rows, err := conn.Query(ctx, `SELECT srvname FROM pg_foreign_server`)
	if err != nil {
		return fmt.Errorf("list foreign servers: %w", err)
	}
	defer rows.Close()

	// Drain rows before executing grants: pgx holds the single connection
	// busy while a result set is open, so Exec on the same conn would fail.
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan foreign server name: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate foreign servers: %w", err)
	}

	for _, name := range names {
		if err := exec(ctx, conn, fmt.Sprintf("GRANT USAGE ON FOREIGN SERVER %s TO %s", ident(name), ident(role))); err != nil {
			return err
		}
	}

	return nil
}

func exec(ctx context.Context, conn Querier, sql string) error {
	if _, err := conn.Exec(ctx, sql); err != nil {
		return fmt.Errorf("exec %q: %w", sql, err)
	}

	return nil
}

func ident(name string) string {
	return pgx.Identifier{name}.Sanitize()
}

func (m *DatabaseManager) logf(format string, args ...any) {
	if m.debug {
		fmt.Printf(format+"\n", args...)
	}
}
