package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// fakeRows is a minimal pgx.Rows stand-in.
type fakeRows struct {
	vals    [][]any
	i       int
	onClose func()
}

func (r *fakeRows) Next() bool {
	r.i++
	if r.i > len(r.vals) {
		// Result set exhausted: the conn is free again, mirroring pgx.
		if r.onClose != nil {
			r.onClose()
		}

		return false
	}

	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.i == 0 || r.i > len(r.vals) {
		return errors.New("scan outside rowset")
	}
	row := r.vals[r.i-1]
	if len(row) != len(dest) {
		return fmt.Errorf("scan count mismatch: %d values, %d dest", len(row), len(dest))
	}
	for j := range dest {
		switch v := row[j].(type) {
		case string:
			s, ok := dest[j].(*string)
			if !ok {
				return fmt.Errorf("dest %d is %T, want *string", j, dest[j])
			}
			*s = v
		case bool:
			b, ok := dest[j].(*bool)
			if !ok {
				return fmt.Errorf("dest %d is %T, want *bool", j, dest[j])
			}
			*b = v
		default:
			return fmt.Errorf("unsupported fake row type %T", row[j])
		}
	}

	return nil
}

func (r *fakeRows) Err() error { return nil }

func (r *fakeRows) Close() {
	if r.onClose != nil {
		r.onClose()
	}
}

// fakeDB routes queries by SQL content and records executed statements.
// busy models pgx's single-connection rule: Exec while a result set is open
// on the same conn must fail.
type fakeDB struct {
	databases      []string
	schemas        []string
	matviews       map[string][]string
	views          map[string][]string
	foreignServers []string
	roles          map[string]bool
	execs          []string
	queries        []string
	busy           bool
	onClose        func()
}

// systemDatabases mirrors the NOT IN filter of the real pg_database query.
var systemDatabases = []string{adminDB, template0, template1, rdsadmin}

func (d *fakeDB) Query(_ context.Context, sql string, args ...any) (Rows, error) {
	d.queries = append(d.queries, sql)

	// A result set is now open on this conn; it stays open until the
	// returned rows are closed.
	d.busy = true
	rows, err := d.query(sql, args...)
	if err != nil {
		return nil, err
	}
	rows.(*fakeRows).onClose = func() { d.busy = false }

	return rows, nil
}

func (d *fakeDB) query(sql string, args ...any) (Rows, error) {
	switch {
	case strings.Contains(sql, "pg_database"):
		rows := make([][]any, 0, len(d.databases))
		for _, db := range d.databases {
			if slices.Contains(systemDatabases, db) {
				continue
			}
			rows = append(rows, []any{db})
		}

		return &fakeRows{vals: rows}, nil
	case strings.Contains(sql, "pg_roles"):
		role, _ := args[0].(string)
		if d.roles[role] {
			return &fakeRows{vals: [][]any{{"1"}}}, nil
		}

		return &fakeRows{}, nil
	case strings.Contains(sql, "information_schema.schemata"):
		rows := make([][]any, 0, len(d.schemas))
		for _, s := range d.schemas {
			if strings.Contains(s, "pg") || s == "information_schema" {
				continue
			}
			rows = append(rows, []any{s})
		}

		return &fakeRows{vals: rows}, nil
	case strings.Contains(sql, "pg_matviews"):
		schema, _ := args[0].(string)

		return &fakeRows{vals: toAny(d.matviews[schema])}, nil
	case strings.Contains(sql, "pg_views"):
		schema, _ := args[0].(string)

		return &fakeRows{vals: toAny(d.views[schema])}, nil
	case strings.Contains(sql, "pg_foreign_server"):

		return &fakeRows{vals: toAny(d.foreignServers)}, nil
	default:
		return nil, fmt.Errorf("fake: unexpected query %q", sql)
	}
}

func (d *fakeDB) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	if d.busy {
		return pgconn.CommandTag{}, errors.New("conn busy")
	}
	d.execs = append(d.execs, sql)

	return pgconn.CommandTag{}, nil
}

func (d *fakeDB) Close(context.Context) error {
	if d.onClose != nil {
		d.onClose()
	}

	return nil
}

func toAny(s []string) [][]any {
	rows := make([][]any, 0, len(s))
	for _, v := range s {
		rows = append(rows, []any{v})
	}

	return rows
}

// fakeConnector hands out fakeDBs per database and records lifecycle.
type fakeConnector struct {
	dbs    map[string]*fakeDB
	opened []string
	closed []string
}

func (c *fakeConnector) Connect(_ context.Context, database string) (Querier, error) {
	c.opened = append(c.opened, database)
	db, ok := c.dbs[database]
	if !ok {
		return nil, fmt.Errorf("fake: no connection for database %q", database)
	}

	return db, nil
}

// testHarness builds a Manager with a fake connector backed by canned data.
func testHarness(databases, schemas []string, existingRoles map[string]bool) (*DatabaseManager, *fakeConnector) {
	if existingRoles == nil {
		existingRoles = map[string]bool{}
	}
	conn := &fakeConnector{dbs: make(map[string]*fakeDB)}
	conn.dbs[adminDB] = &fakeDB{
		databases: databases,
		roles:     existingRoles,
		onClose:   func() { conn.closed = append(conn.closed, adminDB) },
	}
	for _, db := range databases {
		if db == adminDB {
			continue // admin connection already created above
		}
		conn.dbs[db] = &fakeDB{
			schemas:        schemas,
			matviews:       map[string][]string{appSchema: {"post_stats"}},
			views:          map[string][]string{appSchema: {"active_users"}},
			foreignServers: []string{"fdw"},
			roles:          existingRoles,
			onClose:        func() { conn.closed = append(conn.closed, db) },
		}
	}

	return &DatabaseManager{connector: conn}, conn
}

const (
	adminDB   = "postgres"
	template0 = "template0"
	template1 = "template1"
	rdsadmin  = "rdsadmin"
	appSchema = "app"
)

const (
	db1    = "db1"
	db2    = "db2"
	read   = "db1.read"
	rwRole = "db1.readwrite"
)

func hasExec(conn *fakeConnector, database, want string) bool {
	return slices.Contains(conn.dbs[database].execs, want)
}

func execsFor(conn *fakeConnector, database string) []string { return conn.dbs[database].execs }

func TestRefreshRolesPermissions_CreatesRolesForAllDatabases(t *testing.T) {
	t.Parallel()
	m, conn := testHarness([]string{db1, db2}, []string{appSchema}, nil)

	if err := m.RefreshRolesPermissions(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantRoles := []string{`CREATE ROLE "db1.read"`, `CREATE ROLE "db1.readwrite"`, `CREATE ROLE "db2.read"`, `CREATE ROLE "db2.readwrite"`}
	for _, want := range wantRoles {
		if !hasExec(conn, db1, want) && !hasExec(conn, db2, want) {
			t.Errorf("missing exec %q", want)
		}
	}
}

func TestRefreshRolesPermissions_OnlyRequestedDatabases(t *testing.T) {
	t.Parallel()
	m, conn := testHarness([]string{db1, db2}, []string{appSchema}, nil)

	if err := m.RefreshRolesPermissions(context.Background(), []string{db1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !hasExec(conn, db1, `CREATE ROLE "db1.read"`) {
		t.Error("expected role created for db1")
	}
	for _, sql := range execsFor(conn, db2) {
		t.Errorf("unexpected exec on db2: %q", sql)
	}
}

func TestRefreshRolesPermissions_MissingDatabaseFails(t *testing.T) {
	t.Parallel()
	m, _ := testHarness([]string{db1}, []string{appSchema}, nil)

	err := m.RefreshRolesPermissions(context.Background(), []string{"nope"})
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("expected error naming missing database, got %v", err)
	}
}

func TestRefreshRolesPermissions_ExistingRoleNotRecreated(t *testing.T) {
	t.Parallel()
	m, conn := testHarness([]string{db1}, []string{appSchema}, map[string]bool{read: true})

	if err := m.RefreshRolesPermissions(context.Background(), []string{db1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, sql := range execsFor(conn, db1) {
		if sql == `CREATE ROLE "db1.read"` {
			t.Error("existing role was recreated")
		}
	}
	if !hasExec(conn, db1, `GRANT CONNECT ON DATABASE "db1" TO "db1.read"`) {
		t.Error("grants must still be applied to existing role")
	}
}

func TestRefreshRolesPermissions_SecondRunSucceeds(t *testing.T) {
	t.Parallel()
	m, conn := testHarness([]string{db1}, []string{appSchema}, nil)

	if err := m.RefreshRolesPermissions(context.Background(), []string{db1}); err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	// Roles now exist; a second run must not fail or duplicate roles.
	conn.dbs[db1].roles[read] = true
	conn.dbs[db1].roles[rwRole] = true
	if err := m.RefreshRolesPermissions(context.Background(), []string{db1}); err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	creates := 0
	for _, sql := range execsFor(conn, db1) {
		if strings.HasPrefix(sql, "CREATE ROLE") {
			creates++
		}
	}
	if creates != 2 {
		t.Errorf("expected 2 CREATE ROLE total across runs, got %d", creates)
	}
}

func TestRefreshRolesPermissions_SkipsSystemDatabases(t *testing.T) {
	t.Parallel()
	m, conn := testHarness([]string{adminDB, template0, template1, rdsadmin, db1}, []string{appSchema}, nil)

	if err := m.RefreshRolesPermissions(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, db := range []string{"postgres", "template0", "template1", "rdsadmin"} {
		if len(execsFor(conn, db)) > 0 {
			t.Errorf("system database %s was processed", db)
		}
	}
	if !hasExec(conn, db1, `CREATE ROLE "db1.read"`) {
		t.Error("non-system database must be processed")
	}
}

func TestRefreshRolesPermissions_SkipsSystemSchemas(t *testing.T) {
	t.Parallel()
	m, conn := testHarness([]string{db1}, []string{appSchema, "audit", "pg_catalog", "information_schema"}, nil)

	if err := m.RefreshRolesPermissions(context.Background(), []string{db1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, sql := range execsFor(conn, db1) {
		if strings.Contains(sql, `"pg_catalog"`) || strings.Contains(sql, `"information_schema"`) {
			t.Errorf("grant issued on system schema: %q", sql)
		}
	}
	if !hasExec(conn, db1, `GRANT USAGE ON SCHEMA "app" TO "db1.read"`) {
		t.Error("app schema must receive grants")
	}
}

func TestReadRoleGrantsSelectOnly(t *testing.T) {
	t.Parallel()
	m, conn := testHarness([]string{db1}, []string{appSchema}, nil)

	if err := m.RefreshRolesPermissions(context.Background(), []string{db1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	execs := execsFor(conn, db1)
	want := []string{
		`GRANT CONNECT ON DATABASE "db1" TO "db1.read"`,
		`GRANT USAGE ON SCHEMA "app" TO "db1.read"`,
		`GRANT SELECT ON ALL TABLES IN SCHEMA "app" TO "db1.read"`,
		`ALTER DEFAULT PRIVILEGES IN SCHEMA "app" GRANT SELECT ON TABLES TO "db1.read"`,
		`GRANT SELECT ON "app"."post_stats" TO "db1.read"`,
		`GRANT SELECT ON "app"."active_users" TO "db1.read"`,
		`GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA "app" TO "db1.read"`,
		`ALTER DEFAULT PRIVILEGES IN SCHEMA "app" GRANT USAGE, SELECT ON SEQUENCES TO "db1.read"`,
		`GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA "app" TO "db1.read"`,
		`ALTER DEFAULT PRIVILEGES IN SCHEMA "app" GRANT EXECUTE ON FUNCTIONS TO "db1.read"`,
	}
	for _, want := range want {
		if !hasExec(conn, db1, want) {
			t.Errorf("missing read grant %q", want)
		}
	}
	for _, sql := range execs {
		if !strings.Contains(sql, `"db1.read"`) {
			continue // only inspect statements targeting the read role
		}
		if strings.Contains(sql, "INSERT") || strings.Contains(sql, "UPDATE") ||
			strings.Contains(sql, "DELETE") || strings.Contains(sql, "TRUNCATE") ||
			strings.Contains(sql, "OWNER TO") {
			t.Errorf("read role must not get write privileges: %q", sql)
		}
	}
}

func TestReadWriteRoleGrants(t *testing.T) {
	t.Parallel()
	m, conn := testHarness([]string{db1}, []string{appSchema}, nil)

	if err := m.RefreshRolesPermissions(context.Background(), []string{db1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		`GRANT USAGE ON FOREIGN SERVER "fdw" TO "db1.readwrite"`,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON ALL TABLES IN SCHEMA "app" TO "db1.readwrite"`,
		`ALTER DEFAULT PRIVILEGES IN SCHEMA "app" GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON TABLES TO "db1.readwrite"`,
		`GRANT SELECT ON "app"."post_stats" TO "db1.readwrite"`,
		`ALTER MATERIALIZED VIEW "app"."post_stats" OWNER TO "db1.readwrite"`,
		`GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA "app" TO "db1.readwrite"`,
		`GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA "app" TO "db1.readwrite"`,
	}
	for _, want := range want {
		if !hasExec(conn, db1, want) {
			t.Errorf("missing readwrite grant %q", want)
		}
	}
}

func TestRefreshRolesPermissions_ClosesAllConnections(t *testing.T) {
	t.Parallel()
	m, conn := testHarness([]string{db1, db2}, []string{appSchema}, nil)

	if err := m.RefreshRolesPermissions(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantOpened := []string{"postgres", db1, db1, db2, db2}
	if got := conn.opened; len(got) != len(wantOpened) {
		t.Fatalf("expected %d connections (admin + read + readwrite per db), got %d: %v", len(wantOpened), len(got), got)
	}
	if len(conn.closed) != len(conn.opened) {
		t.Errorf("expected all %d connections closed, got %d", len(conn.opened), len(conn.closed))
	}
}
