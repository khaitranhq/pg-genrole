// Package e2e seeds databases with schemas, tables, functions, triggers, views
// and materialized views, runs the pg-genrole binary against them, then verifies
// role permissions as real users.
//
// Requires a PostgreSQL instance already running (e.g. docker compose up -d).
// Run with: go test -tags e2e ./e2e/...
package e2e

import (
	"context"
	"errors"
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
	"github.com/jackc/pgx/v5/pgconn"
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
		conn, err := connect(ctx, "postgres", dbUser, dbPass, false)
		if err == nil {
			_ = conn.Close(ctx)

			return
		}
		time.Sleep(time.Second)
	}
	fmt.Fprintln(os.Stderr, "postgres did not become ready in 60s")
	os.Exit(1)
}

func connect(ctx context.Context, database, user, password string, simple bool) (*pgx.Conn, error) {
	cfg, err := pgx.ParseConfig("")
	if err != nil {
		return nil, fmt.Errorf("parse pgx config: %w", err)
	}
	cfg.Host = dbHost
	cfg.Port = dbPort
	cfg.User = user
	cfg.Password = password
	cfg.Database = database
	if simple {
		// init.sql holds many statements; only the simple protocol runs them
		// in one Exec.
		cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	}
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to %s as %s: %w", database, user, err)
	}

	return conn, nil
}

// mustConnect connects and registers cleanup on the test.
func mustConnect(t *testing.T, database, user, password string) *pgx.Conn {
	t.Helper()
	conn, err := connect(context.Background(), database, user, password, false)
	if err != nil {
		t.Fatalf("connect to %s as %s: %v", database, user, err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	return conn
}

func ident(name string) string {
	return pgx.Identifier{name}.Sanitize()
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
	if err == nil {
		// pgx reports execution errors lazily via rows.Err(), not at Query().
		for rows.Next() {
		}
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		t.Fatalf("query %q: %v", sql, err)
	}
}

// expectDenied asserts the statement fails with a permission error.
func expectDenied(t *testing.T, conn *pgx.Conn, sql string) {
	t.Helper()
	rows, err := conn.Query(context.Background(), sql)
	if err == nil {
		// pgx reports execution errors lazily via rows.Err(), not at Query().
		for rows.Next() {
		}
		rows.Close()
		err = rows.Err()
	}
	if err == nil {
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
	// Denials surface as SQLSTATE 42501 (insufficient_privilege), with
	// messages like "permission denied for table x" or "must be owner of y".
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("expected permission denied for %q, got: %v", sql, err)
	}
}

// setup recreates test users and databases with full schema, functions,
// triggers, views, materialized views and sample data.
func setup(t *testing.T, dbs []string) {
	t.Helper()
	admin := mustConnect(t, "postgres", dbUser, dbPass)
	// Drop databases before roles: roles may own objects inside them.
	for _, db := range dbs {
		execSQL(t, admin, "DROP DATABASE IF EXISTS "+db)
	}
	// Drop stale genrole roles from previous tests so each test starts clean.
	for _, db := range testDBs {
		for _, suffix := range []string{".read", ".readwrite"} {
			execSQL(t, admin, fmt.Sprintf("DROP ROLE IF EXISTS %s", ident(db+suffix)))
		}
	}
	for _, u := range []string{readUser, rwUser} {
		execSQL(t, admin, "DROP ROLE IF EXISTS "+u)
		execSQL(t, admin, fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", u, testPass))
	}
	seed, err := os.ReadFile("init.sql")
	if err != nil {
		t.Fatalf("read init.sql: %v", err)
	}
	// init.sql bootstraps its own database for docker-entrypoint-initdb.d
	// (CREATE DATABASE + \c header). The test creates and connects to each
	// test database itself, so drop the header before Exec.
	lines := strings.Split(string(seed), "\n")
	body := seed
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "\\") {
			body = []byte(strings.Join(lines[i+1:], "\n"))
			break
		}
	}
	for _, db := range dbs {
		execSQL(t, admin, "CREATE DATABASE "+db)
		// By default PUBLIC can connect to any database; revoke so only
		// roles granted CONNECT by genrole (and superusers) can connect.
		execSQL(t, admin, fmt.Sprintf("REVOKE CONNECT ON DATABASE %s FROM PUBLIC", db))
		conn, err := connect(context.Background(), db, dbUser, dbPass, true)
		if err != nil {
			t.Fatalf("connect to %s: %v", db, err)
		}
		if _, err := conn.Exec(context.Background(), string(body)); err != nil {
			t.Fatalf("seed %s: %v", db, err)
		}
		_ = conn.Close(context.Background())
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
func TestDefaultPrivileges(t *testing.T) {
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

		rw := mustConnect(t, db, rwUser, testPass)
		mustQuery(t, rw, "SELECT note FROM app.new_table")
		execSQL(t, rw, "INSERT INTO app.new_table (id, note) VALUES (2, 'yes')")
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
	if conn, err := connect(context.Background(), db3, readUser, testPass, false); err == nil {
		_ = conn.Close(context.Background())
		t.Errorf("expected permission denied connecting to db3, got nil error")
	} else if !strings.Contains(err.Error(), "permission denied") {
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
