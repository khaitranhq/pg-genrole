// Package e2e seeds databases with schemas, tables, functions, triggers, views
// and materialized views, runs the pg-genrole binary against them, then verifies
// role permissions as real users.
//
// Requires a PostgreSQL instance already running (e.g. docker compose up -d).
// Run with: go test -tags e2e ./e2e/...
package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	dbHost = "127.0.0.1"
	dbPort = 5432
	dbUser = "postgres"
	dbPass = "postgres"

	readUser = "test_read_user"
	rwUser   = "test_readwrite_user"
	testPass = "test_password"

	db1 = "db1"
	db2 = "db2"
	db3 = "db3"
)

var testDBs = []string{db1, db2, db3}

var genroleBin string

func TestMain(m *testing.M) {
	ctx := context.Background()

	waitForPostgres()

	genroleBin = filepath.Join(os.TempDir(), "pg-genrole-e2e")
	if err := run(ctx, "go", "build", "-o", genroleBin, "../cmd/pg-genrole"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.Remove(genroleBin)
	os.Exit(code)
}

func run(ctx context.Context, name string, args ...string) error {
	//nolint:gosec // e2e deliberately builds and runs the pg-genrole binary
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", name, err)
	}

	return nil
}

func waitForPostgres() {
	ctx := context.Background()
	for range 60 {
		conn, err := connect(ctx, "postgres", dbUser, dbPass)
		if err == nil {
			_ = conn.Close(ctx)

			return
		}
		time.Sleep(time.Second)
	}
	fmt.Fprintln(os.Stderr, "postgres did not become ready in 60s")
	os.Exit(1)
}

func connect(ctx context.Context, database, user, password string) (*pgx.Conn, error) {
	cfg, err := pgx.ParseConfig("")
	if err != nil {
		return nil, fmt.Errorf("parse pgx config: %w", err)
	}
	cfg.Host = dbHost
	cfg.Port = dbPort
	cfg.User = user
	cfg.Password = password
	cfg.Database = database
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to %s as %s: %w", database, user, err)
	}

	return conn, nil
}

// mustConnect connects and registers cleanup on the test.
func mustConnect(t *testing.T, database, user, password string) *pgx.Conn {
	t.Helper()
	conn, err := connect(context.Background(), database, user, password)
	if err != nil {
		t.Fatalf("connect to %s as %s: %v", database, user, err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	return conn
}

func execSQL(t *testing.T, conn *pgx.Conn, sql string) {
	t.Helper()
	if _, err := conn.Exec(context.Background(), sql); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func mustQuery(t *testing.T, conn *pgx.Conn, sql string) {
	t.Helper()
	rows, err := conn.Query(context.Background(), sql)
	if err != nil {
		t.Fatalf("query %q: %v", sql, err)
	}
	rows.Close()
}

// expectDenied asserts the statement fails with a permission error.
func expectDenied(t *testing.T, conn *pgx.Conn, sql string) {
	t.Helper()
	rows, err := conn.Query(context.Background(), sql)
	if err == nil {
		rows.Close()
		dbg, qerr := conn.Query(context.Background(), `SELECT current_user, current_database(), has_table_privilege('app.users','INSERT'), pg_has_role('db1.read','MEMBER'), pg_has_role('db1.readwrite','MEMBER')`)
		if qerr == nil {
			var cu, cd string
			var ins, r1, r2 bool
			for dbg.Next() {
				//nolint:gosec // debug-only scan; error is not actionable
				dbg.Scan(&cu, &cd, &ins, &r1, &r2)
				t.Logf("DEBUG user=%s db=%s insert=%v member_read=%v member_rw=%v", cu, cd, ins, r1, r2)
			}
			dbg.Close()
		}
		t.Fatalf("expected permission denied for %q, but it succeeded", sql)
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected permission denied for %q, got: %v", sql, err)
	}
}

// assertServerPrivilege asserts whether conn's user has USAGE on a foreign server.
func assertServerPrivilege(t *testing.T, conn *pgx.Conn, server string, want bool) {
	t.Helper()
	var got bool
	if err := conn.QueryRow(context.Background(),
		"SELECT has_server_privilege(current_user, $1, 'USAGE')", server).Scan(&got); err != nil {
		t.Fatalf("check USAGE on foreign server %s: %v", server, err)
	}
	if got != want {
		t.Errorf("USAGE on foreign server %s = %v, want %v", server, got, want)
	}
}

// setup recreates test users and databases with full schema, functions,
// triggers, views, materialized views and sample data.
func setup(t *testing.T, dbs []string) {
	t.Helper()
	admin := mustConnect(t, "postgres", dbUser, dbPass)
	for _, u := range []string{readUser, rwUser} {
		execSQL(t, admin, "DROP ROLE IF EXISTS "+u)
		execSQL(t, admin, fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", u, testPass))
	}
	// Cluster-wide objects used by the foreign-server permission checks.
	execSQL(t, admin, "CREATE FOREIGN DATA WRAPPER IF NOT EXISTS dummy_fdw")
	execSQL(t, admin, "CREATE SERVER IF NOT EXISTS dummy_server FOREIGN DATA WRAPPER dummy_fdw")
	for _, db := range dbs {
		execSQL(t, admin, "DROP DATABASE IF EXISTS "+db)
		execSQL(t, admin, "CREATE DATABASE "+db)
		conn := mustConnect(t, db, dbUser, dbPass)
		for _, stmt := range seedStatements(db) {
			execSQL(t, conn, stmt)
		}
	}
}

// runGenrole executes the pg-genrole CLI, optionally restricted to dbs.
func runGenrole(t *testing.T, dbs ...string) {
	t.Helper()
	args := []string{
		"-host", dbHost,
		"-port", strconv.Itoa(dbPort),
		"-user", dbUser,
		"-password", dbPass,
	}
	if len(dbs) > 0 {
		args = append(args, "-databases", strings.Join(dbs, ","))
	}
	//nolint:gosec // e2e deliberately runs the pg-genrole binary
	cmd := exec.CommandContext(context.Background(), genroleBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pg-genrole failed: %v\n%s", err, out)
	}
}

func grantRoles(t *testing.T, dbs []string) {
	t.Helper()
	admin := mustConnect(t, "postgres", dbUser, dbPass)
	for _, db := range dbs {
		execSQL(t, admin, fmt.Sprintf(`GRANT "%s.read" TO %s`, db, readUser))
		execSQL(t, admin, fmt.Sprintf(`GRANT "%s.readwrite" TO %s`, db, rwUser))
	}
}

func existingRoles(t *testing.T) []string {
	t.Helper()
	admin := mustConnect(t, "postgres", dbUser, dbPass)
	rows, err := admin.Query(context.Background(), "SELECT rolname FROM pg_roles")
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	defer rows.Close()
	var roles []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan role: %v", err)
		}
		roles = append(roles, name)
	}

	return roles
}

//nolint:paralleltest // e2e tests share one dockerized postgres; cannot run in parallel
func TestGenerateRolesForAllDatabases(t *testing.T) {
	setup(t, testDBs)
	runGenrole(t)
	grantRoles(t, testDBs)

	roles := existingRoles(t)
	for _, db := range testDBs {
		for _, suffix := range []string{".read", ".readwrite"} {
			want := db + suffix
			if !slices.Contains(roles, want) {
				t.Errorf("role %s not created", want)
			}
		}
	}
}

//nolint:paralleltest // e2e tests share one dockerized postgres; cannot run in parallel
func TestReadRolePermissions(t *testing.T) {
	setup(t, testDBs)
	runGenrole(t)
	grantRoles(t, testDBs)

	for _, db := range testDBs {
		conn := mustConnect(t, db, readUser, testPass)

		mustQuery(t, conn, "SELECT email FROM app.users LIMIT 1")
		expectDenied(t, conn, "INSERT INTO app.users (email, username, role) VALUES ('x@y.z', 'x', 'user')")

		mustQuery(t, conn, "SELECT post_count FROM app.get_user_stats(1)")
		mustQuery(t, conn, "SELECT username FROM app.active_users LIMIT 1")
		expectDenied(t, conn, "CREATE OR REPLACE VIEW app.active_users AS SELECT 1 AS dummy")

		mustQuery(t, conn, "SELECT status FROM app.post_stats LIMIT 1")
		expectDenied(t, conn, "REFRESH MATERIALIZED VIEW app.post_stats")

		mustQuery(t, conn, "SELECT last_value FROM app.user_id_seq")
		expectDenied(t, conn, "ALTER SEQUENCE app.user_id_seq RESTART WITH 1")
	}
}

//nolint:paralleltest // e2e tests share one dockerized postgres; cannot run in parallel
func TestReadWriteRolePermissions(t *testing.T) {
	setup(t, testDBs)
	runGenrole(t)
	grantRoles(t, testDBs)

	for _, db := range testDBs {
		conn := mustConnect(t, db, rwUser, testPass)

		var email, username, role string
		err := conn.QueryRow(
			context.Background(),
			"INSERT INTO app.users (email, username, role) VALUES ('test@test.com', 'test_user', 'user') RETURNING email, username, role",
		).Scan(&email, &username, &role)
		if err != nil {
			t.Fatalf("insert as readwrite user: %v", err)
		}
		if email != "test@test.com" || username != "test_user" || role != "user" {
			t.Errorf("unexpected inserted row: %s/%s/%s", email, username, role)
		}
		execSQL(t, conn, "UPDATE app.users SET username = 'updated_user' WHERE email = 'test@test.com'")
		var got string
		if err := conn.QueryRow(context.Background(), "SELECT username FROM app.users WHERE email = 'test@test.com'").Scan(&got); err != nil {
			t.Fatalf("select updated user: %v", err)
		}
		if got != "updated_user" {
			t.Errorf("update did not persist, got %q", got)
		}
		execSQL(t, conn, "DELETE FROM app.users WHERE email = 'test@test.com'")

		mustQuery(t, conn, "SELECT username FROM app.active_users LIMIT 1")
		expectDenied(t, conn, "CREATE OR REPLACE VIEW app.test_view AS SELECT 1 AS dummy")

		mustQuery(t, conn, "SELECT status FROM app.post_stats LIMIT 1")
		execSQL(t, conn, "REFRESH MATERIALIZED VIEW app.post_stats")
		// Owner of the matview: can ALTER it (rename and back).
		execSQL(t, conn, "ALTER MATERIALIZED VIEW app.post_stats RENAME TO app.post_stats_renamed")
		execSQL(t, conn, "ALTER MATERIALIZED VIEW app.post_stats_renamed RENAME TO app.post_stats")

		mustQuery(t, conn, "SELECT post_count FROM app.get_user_stats(1)")
		mustQuery(t, conn, "SELECT last_value FROM app.user_id_seq")
		expectDenied(t, conn, "ALTER SEQUENCE app.user_id_seq RESTART WITH 1")

		expectDenied(t, conn, "CREATE SCHEMA test_schema")
		expectDenied(t, conn, "ALTER SCHEMA app RENAME TO test_app")

		// Trigger sets updated_at on insert and refresh on update.
		var insertedAt, updatedAt time.Time
		if err := conn.QueryRow(
			context.Background(),
			"INSERT INTO app.users (email, username, role) VALUES ('trigger@test.com', 'trigger_test', 'user') RETURNING updated_at",
		).Scan(&insertedAt); err != nil {
			t.Fatalf("insert for trigger test: %v", err)
		}
		time.Sleep(time.Second)
		if err := conn.QueryRow(
			context.Background(),
			"UPDATE app.users SET username = 'trigger_updated' WHERE email = 'trigger@test.com' RETURNING updated_at",
		).Scan(&updatedAt); err != nil {
			t.Fatalf("update for trigger test: %v", err)
		}
		if !updatedAt.After(insertedAt) {
			t.Errorf("trigger did not refresh updated_at (%v -> %v)", insertedAt, updatedAt)
		}
		execSQL(t, conn, "DELETE FROM app.users WHERE email = 'trigger@test.com'")
	}
}

//nolint:paralleltest // e2e tests share one dockerized postgres; cannot run in parallel
func TestDefaultPrivilegesAndForeignServers(t *testing.T) {
	setup(t, testDBs)
	runGenrole(t)
	grantRoles(t, testDBs)

	for _, db := range testDBs {
		admin := mustConnect(t, db, dbUser, dbPass)
		// Created after genrole: default privileges must cover it.
		execSQL(t, admin, "CREATE TABLE app.new_table (id INTEGER PRIMARY KEY, note TEXT)")
		execSQL(t, admin, "INSERT INTO app.new_table (id, note) VALUES (1, 'seed')")

		read := mustConnect(t, db, readUser, testPass)
		mustQuery(t, read, "SELECT note FROM app.new_table")
		expectDenied(t, read, "INSERT INTO app.new_table (id, note) VALUES (2, 'no')")
		assertServerPrivilege(t, read, "dummy_server", false)

		rw := mustConnect(t, db, rwUser, testPass)
		mustQuery(t, rw, "SELECT note FROM app.new_table")
		execSQL(t, rw, "INSERT INTO app.new_table (id, note) VALUES (2, 'yes')")
		assertServerPrivilege(t, rw, "dummy_server", true)
	}
}

//nolint:paralleltest // e2e tests share one dockerized postgres; cannot run in parallel
func TestMatviewOwnership(t *testing.T) {
	setup(t, testDBs)
	runGenrole(t)
	grantRoles(t, testDBs)

	for _, db := range testDBs {
		conn := mustConnect(t, db, dbUser, dbPass)
		var owner string
		if err := conn.QueryRow(
			context.Background(),
			"SELECT matviewowner FROM pg_matviews WHERE schemaname = 'app' AND matviewname = 'post_stats'",
		).Scan(&owner); err != nil {
			t.Fatalf("query matview owner: %v", err)
		}
		if want := db + ".readwrite"; owner != want {
			t.Errorf("matview owner = %q, want %q", owner, want)
		}
	}
}

//nolint:paralleltest // e2e tests share one dockerized postgres; cannot run in parallel
func TestSpecificDatabases(t *testing.T) {
	setup(t, testDBs)
	runGenrole(t, db1, db2)
	grantRoles(t, []string{db1, db2})

	roles := existingRoles(t)
	for _, want := range []string{"db1.read", "db1.readwrite", "db2.read", "db2.readwrite"} {
		if !slices.Contains(roles, want) {
			t.Errorf("role %s not created", want)
		}
	}
	for _, unwanted := range []string{"db3.read", "db3.readwrite"} {
		if slices.Contains(roles, unwanted) {
			t.Errorf("role %s must not be created", unwanted)
		}
	}

	mustQuery(t, mustConnect(t, db1, readUser, testPass), "SELECT email FROM app.users LIMIT 1")

	// db3 was not processed: read user cannot even connect.
	if _, err := connect(context.Background(), db3, readUser, testPass); err == nil ||
		!strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected permission denied connecting to db3, got %v", err)
	}
}

//nolint:paralleltest // e2e tests share one dockerized postgres; cannot run in parallel
func TestRefreshIsIdempotent(t *testing.T) {
	setup(t, testDBs)
	runGenrole(t, db1, db2)
	grantRoles(t, []string{db1, db2})
	runGenrole(t, db1, db2) // second run must succeed and keep roles

	roles := existingRoles(t)
	for _, want := range []string{"db1.read", "db1.readwrite", "db2.read", "db2.readwrite"} {
		if !slices.Contains(roles, want) {
			t.Errorf("role %s lost after refresh", want)
		}
	}
	mustQuery(t, mustConnect(t, db1, readUser, testPass), "SELECT email FROM app.users LIMIT 1")
}

// seedStatements returns the DDL and sample data for one database, mirroring
// the fixtures of the original TypeScript integration tests.
func seedStatements(database string) []string {
	return []string{
		"CREATE SCHEMA IF NOT EXISTS app",
		"CREATE SCHEMA IF NOT EXISTS audit",
		`CREATE SEQUENCE app.user_id_seq INCREMENT 1 START 1000 MINVALUE 1000 MAXVALUE 9999999999`,
		`CREATE SEQUENCE app.post_id_seq INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 9999999999`,
		`CREATE TABLE app.users (
			id INTEGER PRIMARY KEY DEFAULT nextval('app.user_id_seq'),
			email VARCHAR(255) UNIQUE NOT NULL,
			username VARCHAR(50) NOT NULL,
			role VARCHAR(20) DEFAULT 'user',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE app.posts (
			id INTEGER PRIMARY KEY DEFAULT nextval('app.post_id_seq'),
			user_id INTEGER REFERENCES app.users(id),
			title VARCHAR(200) NOT NULL,
			content TEXT,
			status VARCHAR(20) DEFAULT 'draft',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE app.comments (
			id SERIAL PRIMARY KEY,
			post_id INTEGER REFERENCES app.posts(id),
			user_id INTEGER REFERENCES app.users(id),
			content TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE audit.changes (
			id SERIAL PRIMARY KEY,
			table_name VARCHAR(50),
			record_id INTEGER,
			action VARCHAR(20),
			changed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			old_data JSONB,
			new_data JSONB
		)`,
		`CREATE OR REPLACE FUNCTION update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql`,
		`CREATE OR REPLACE FUNCTION audit.log_changes()
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
		$$ LANGUAGE plpgsql`,
		`CREATE OR REPLACE FUNCTION app.get_user_stats(p_user_id INTEGER)
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
		$$ LANGUAGE plpgsql`,
		`CREATE TRIGGER update_users_modtime BEFORE UPDATE ON app.users
		FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,
		`CREATE TRIGGER update_posts_modtime BEFORE UPDATE ON app.posts
		FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,
		`CREATE TRIGGER audit_users_changes AFTER UPDATE OR DELETE ON app.users
		FOR EACH ROW EXECUTE FUNCTION audit.log_changes()`,
		`CREATE TRIGGER audit_posts_changes AFTER UPDATE OR DELETE ON app.posts
		FOR EACH ROW EXECUTE FUNCTION audit.log_changes()`,
		`CREATE VIEW app.active_users AS
		SELECT u.id, u.username, u.email,
			COUNT(DISTINCT p.id) as post_count,
			COUNT(DISTINCT c.id) as comment_count,
			MAX(GREATEST(p.created_at, c.created_at)) as last_activity
		FROM app.users u
		LEFT JOIN app.posts p ON p.user_id = u.id
		LEFT JOIN app.comments c ON c.user_id = u.id
		GROUP BY u.id, u.username, u.email`,
		`CREATE MATERIALIZED VIEW app.post_stats AS
		SELECT date_trunc('day', created_at) as post_date, status,
			COUNT(*) as post_count, COUNT(DISTINCT user_id) as unique_authors
		FROM app.posts
		GROUP BY date_trunc('day', created_at), status
		WITH DATA`,
		"CREATE INDEX idx_posts_user_id ON app.posts(user_id)",
		"CREATE INDEX idx_comments_post_id ON app.comments(post_id)",
		"CREATE INDEX idx_comments_user_id ON app.comments(user_id)",
		"CREATE INDEX idx_post_stats_date ON app.post_stats(post_date)",
		fmt.Sprintf(`INSERT INTO app.users (email, username, role) VALUES
			('user1@%s.com', 'user1_%s', 'admin'),
			('user2@%s.com', 'user2_%s', 'user'),
			('user3@%s.com', 'user3_%s', 'user')`, database, database, database, database, database, database),
		`INSERT INTO app.posts (user_id, title, content, status)
		SELECT u.id, 'Post ' || generate_series || ' by ' || u.username,
			'Content for post ' || generate_series || ' in database',
			CASE WHEN generate_series % 2 = 0 THEN 'published' ELSE 'draft' END
		FROM app.users u CROSS JOIN generate_series(1, 3)`,
		`INSERT INTO app.comments (post_id, user_id, content)
		SELECT p.id, u.id, 'Comment on post ' || p.id || ' by ' || u.username
		FROM app.posts p CROSS JOIN app.users u
		WHERE p.status = 'published'`,
		"REFRESH MATERIALIZED VIEW app.post_stats",
	}
}
