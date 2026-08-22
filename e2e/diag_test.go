// Diagnostic test: this file helped figure out why TestReadRolePermissions
// failed even though the roles were correct.
//
// The problem: pgx does not return the error right away when a query is
// denied. Query() returns nil, and the error only appears later when reading
// the rows (rows.Next() / rows.Err()). The old expectDenied helper checked
// only the first error, so it thought the INSERT worked when it was actually
// denied.
//
// The fix: expectDenied and mustQuery now drain the rows and check rows.Err().
package e2e

import (
	"context"
	"testing"
)

//nolint:paralleltest // diagnostic
func TestDiagRepro(t *testing.T) {
	setup(t, testDBs)
	runGenrole(t)
	grantRoles(t, testDBs)

	ctx := context.Background()
	db := db1
	admin := mustConnect(t, db, dbUser, dbPass)
	execSQL(t, admin, "CREATE TABLE app.new_table (id INTEGER PRIMARY KEY, note TEXT)")
	execSQL(t, admin, "INSERT INTO app.new_table (id, note) VALUES (1, 'seed')")

	read := mustConnect(t, db, readUser, testPass)
	mustQuery(t, read, "SELECT note FROM app.new_table")

	rows, errQ := read.Query(ctx, "INSERT INTO app.new_table (id, note) VALUES (2, 'no')")
	t.Logf("Query() initial err = %v", errQ)
	next := rows.Next()
	t.Logf("rows.Next() = %v", next)
	t.Logf("rows.Err() = %v", rows.Err())
	rows.Close()

	// Did the row actually get inserted?
	var cnt int
	if err := admin.QueryRow(ctx, "SELECT count(*) FROM app.new_table WHERE id = 2").Scan(&cnt); err != nil {
		t.Fatalf("count inserted rows: %v", err)
	}
	t.Logf("rows with id=2 after attempted INSERT = %d", cnt)
}
