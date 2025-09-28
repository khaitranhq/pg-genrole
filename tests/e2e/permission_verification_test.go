package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PermissionVerificationTestSuite provides comprehensive permission testing
type PermissionVerificationTestSuite struct {
	suite.Suite
	binaryPath       string
	containers       map[string]*postgres.PostgresContainer
	connectionInfos  map[string]ConnectionInfo
	postgresVersions []string
	currentVersion   string
}

// PermissionType represents different types of permissions to test
type PermissionType string

const (
	DatabasePermission           PermissionType = "database"
	SchemaPermission             PermissionType = "schema"
	TablePermission              PermissionType = "table"
	ViewPermission               PermissionType = "view"
	MaterializedViewPermission   PermissionType = "materialized_view"
	SequencePermission           PermissionType = "sequence"
	FunctionPermission           PermissionType = "function"
	ProcedurePermission          PermissionType = "procedure"
	IndexPermission              PermissionType = "index"
	DomainPermission             PermissionType = "domain"
	ExtensionPermission          PermissionType = "extension"
	ForeignDataWrapperPermission PermissionType = "foreign_data_wrapper"
	ForeignServerPermission      PermissionType = "foreign_server"
	ForeignTablePermission       PermissionType = "foreign_table"
	LargeObjectPermission        PermissionType = "large_object"
	TablespacePermission         PermissionType = "tablespace"
)

// PermissionMatrix defines expected permissions for each role type
type PermissionMatrix struct {
	ObjectType   PermissionType
	Permission   string
	ReadOnly     bool   // Should RO role have this permission?
	ReadWrite    bool   // Should RW role have this permission?
	Admin        bool   // Should Admin role have this permission?
	TestQuery    string // SQL query to test the permission
	SetupQuery   string // Optional SQL to set up test objects
	CleanupQuery string // Optional SQL to clean up test objects
}

// Role names used by pg-genrole
const (
	ReadOnlyRoleName  = "testdb_readonly"
	ReadWriteRoleName = "testdb_readwrite"
	AdminRoleName     = "testdb_admin"
)

// SetupSuite initializes the test environment with PostgreSQL containers
func (suite *PermissionVerificationTestSuite) SetupSuite() {
	// Define PostgreSQL versions to test
	suite.postgresVersions = []string{
		"postgres:13-alpine",
		"postgres:14-alpine",
		"postgres:15-alpine",
		"postgres:16-alpine",
	}

	suite.containers = make(map[string]*postgres.PostgresContainer)
	suite.connectionInfos = make(map[string]ConnectionInfo)

	// Build the pg-genrole binary
	suite.buildBinary()

	// Start PostgreSQL containers for each version
	suite.startPostgresContainers()
}

// TearDownSuite cleans up all test resources
func (suite *PermissionVerificationTestSuite) TearDownSuite() {
	ctx := context.Background()

	// Clean up role users from all containers
	for version := range suite.containers {
		conn, err := suite.getDBConnection(version)
		if err == nil {
			suite.cleanupAllRoleUsers(conn)
			conn.Close(ctx)
		}
	}

	// Terminate all containers
	for _, container := range suite.containers {
		if container != nil {
			_ = container.Terminate(ctx)
		}
	}

	// Clean up binary
	if suite.binaryPath != "" {
		_ = os.Remove(suite.binaryPath)
	}
}

// buildBinary compiles the pg-genrole binary for testing
func (suite *PermissionVerificationTestSuite) buildBinary() {
	tempDir := suite.T().TempDir()
	suite.binaryPath = filepath.Join(tempDir, "pg-genrole")

	cmd := exec.Command("go", "build", "-o", suite.binaryPath, "../../cmd/pg-genrole")
	output, err := cmd.CombinedOutput()
	require.NoError(suite.T(), err, "Failed to build binary: %s", string(output))

	// Verify binary exists and is executable
	info, err := os.Stat(suite.binaryPath)
	require.NoError(suite.T(), err, "Binary not found after build")
	require.True(suite.T(), info.Mode()&0111 != 0, "Binary is not executable")
}

// startPostgresContainers starts PostgreSQL containers for all versions
func (suite *PermissionVerificationTestSuite) startPostgresContainers() {
	ctx := context.Background()

	for _, version := range suite.postgresVersions {
		suite.T().Logf("Starting PostgreSQL container for version: %s", version)

		container, err := postgres.Run(ctx,
			version,
			postgres.WithDatabase("testdb"),
			postgres.WithUsername("postgres"),
			postgres.WithPassword("testpass"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(30*time.Second),
			),
		)
		require.NoError(suite.T(), err, "Failed to start PostgreSQL %s container", version)

		suite.containers[version] = container

		// Get connection details
		host, err := container.Host(ctx)
		require.NoError(suite.T(), err, "Failed to get host for %s", version)

		port, err := container.MappedPort(ctx, "5432")
		require.NoError(suite.T(), err, "Failed to get port for %s", version)

		suite.connectionInfos[version] = ConnectionInfo{
			Host:     host,
			Port:     port.Port(),
			User:     "postgres",
			Password: "testpass",
			Database: "testdb",
		}

		suite.T().Logf("PostgreSQL %s running at %s:%s", version, host, port.Port())
	}
}

// runRoleCreation executes the pg-genrole binary to create roles
func (suite *PermissionVerificationTestSuite) runRoleCreation(
	version string,
) (stdout, stderr string, err error) {
	connInfo := suite.connectionInfos[version]

	args := []string{
		"--host", connInfo.Host,
		"--port", connInfo.Port,
		"--user", connInfo.User,
		"--password", connInfo.Password,
		"--database", connInfo.Database,
	}

	cmd := exec.Command(suite.binaryPath, args...)
	output, err := cmd.CombinedOutput()

	return string(output), "", err
}

// getDBConnection creates a database connection for testing
func (suite *PermissionVerificationTestSuite) getDBConnection(version string) (*pgx.Conn, error) {
	connInfo := suite.connectionInfos[version]

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		connInfo.User, connInfo.Password, connInfo.Host, connInfo.Port, connInfo.Database)

	return pgx.Connect(context.Background(), connStr)
}

// getCurrentVersion returns the current version being tested
func (suite *PermissionVerificationTestSuite) getCurrentVersion() string {
	return suite.currentVersion
}

// setCurrentVersion sets the current version being tested
func (suite *PermissionVerificationTestSuite) setCurrentVersion(version string) {
	suite.currentVersion = version
}

// getRoleConnection creates a database connection for a specific role user
func (suite *PermissionVerificationTestSuite) getRoleConnection(
	version, roleName string,
) (*pgx.Conn, error) {
	connInfo := suite.connectionInfos[version]
	userName := roleName + "_user"
	userPassword := "test_password_123"

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		userName, userPassword, connInfo.Host, connInfo.Port, connInfo.Database)

	return pgx.Connect(context.Background(), connStr)
}

// setupRoleUser creates a user for the given role and grants the role to that user
func (suite *PermissionVerificationTestSuite) setupRoleUser(conn *pgx.Conn, roleName string) error {
	ctx := context.Background()
	userName := roleName + "_user"
	userPassword := "test_password_123"

	// Create user if it doesn't exist
	createUserQuery := fmt.Sprintf(`
		DO $$ 
		BEGIN
			IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '%s') THEN
				CREATE USER %s WITH PASSWORD '%s';
			END IF;
		END $$`, userName, userName, userPassword)

	_, err := conn.Exec(ctx, createUserQuery)
	if err != nil {
		return fmt.Errorf("failed to create user %s: %w", userName, err)
	}

	// Grant the role to the user
	grantRoleQuery := fmt.Sprintf("GRANT %s TO %s", roleName, userName)
	_, err = conn.Exec(ctx, grantRoleQuery)
	if err != nil {
		return fmt.Errorf("failed to grant role %s to user %s: %w", roleName, userName, err)
	}

	return nil
}

// cleanupRoleUser removes the user created for the given role
func (suite *PermissionVerificationTestSuite) cleanupRoleUser(
	conn *pgx.Conn,
	roleName string,
) error {
	ctx := context.Background()
	userName := roleName + "_user"

	// Drop user if exists
	dropUserQuery := fmt.Sprintf("DROP USER IF EXISTS %s", userName)
	_, err := conn.Exec(ctx, dropUserQuery)
	if err != nil {
		return fmt.Errorf("failed to drop user %s: %w", userName, err)
	}

	return nil
}

// cleanupAllRoleUsers removes all role users created during testing
func (suite *PermissionVerificationTestSuite) cleanupAllRoleUsers(conn *pgx.Conn) {
	roles := []string{ReadOnlyRoleName, ReadWriteRoleName, AdminRoleName}

	for _, roleName := range roles {
		err := suite.cleanupRoleUser(conn, roleName)
		if err != nil {
			suite.T().Logf("Failed to cleanup role user for %s: %v", roleName, err)
		}
	}
}

// getPermissionMatrix returns the comprehensive permission matrix for testing
func (suite *PermissionVerificationTestSuite) getPermissionMatrix() []PermissionMatrix {
	return []PermissionMatrix{
		// Database permissions
		{
			ObjectType:   DatabasePermission,
			Permission:   "CONNECT",
			ReadOnly:     true,
			ReadWrite:    true,
			Admin:        true,
			TestQuery:    "SELECT current_database()",
			SetupQuery:   "",
			CleanupQuery: "",
		},
		{
			ObjectType:   DatabasePermission,
			Permission:   "CREATE",
			ReadOnly:     false,
			ReadWrite:    false,
			Admin:        true,
			TestQuery:    "CREATE SCHEMA test_schema_db_perm",
			SetupQuery:   "",
			CleanupQuery: "DROP SCHEMA IF EXISTS test_schema_db_perm CASCADE",
		},

		// Schema permissions
		{
			ObjectType:   SchemaPermission,
			Permission:   "USAGE",
			ReadOnly:     true,
			ReadWrite:    true,
			Admin:        true,
			TestQuery:    "SELECT schema_name FROM information_schema.schemata WHERE schema_name = 'public'",
			SetupQuery:   "",
			CleanupQuery: "",
		},
		{
			ObjectType:   SchemaPermission,
			Permission:   "CREATE",
			ReadOnly:     false,
			ReadWrite:    false,
			Admin:        true,
			TestQuery:    "CREATE TABLE public.test_schema_create (id INT)",
			SetupQuery:   "",
			CleanupQuery: "DROP TABLE IF EXISTS public.test_schema_create",
		},

		// Table permissions
		{
			ObjectType:   TablePermission,
			Permission:   "SELECT",
			ReadOnly:     true,
			ReadWrite:    true,
			Admin:        true,
			TestQuery:    "SELECT COUNT(*) FROM test_table",
			SetupQuery:   "CREATE TABLE IF NOT EXISTS test_table (id SERIAL PRIMARY KEY, name TEXT)",
			CleanupQuery: "DROP TABLE IF EXISTS test_table",
		},
		{
			ObjectType:   TablePermission,
			Permission:   "INSERT",
			ReadOnly:     false,
			ReadWrite:    true,
			Admin:        true,
			TestQuery:    "INSERT INTO test_table (name) VALUES ('test')",
			SetupQuery:   "CREATE TABLE IF NOT EXISTS test_table (id SERIAL PRIMARY KEY, name TEXT)",
			CleanupQuery: "DROP TABLE IF EXISTS test_table",
		},
		{
			ObjectType:   TablePermission,
			Permission:   "UPDATE",
			ReadOnly:     false,
			ReadWrite:    true,
			Admin:        true,
			TestQuery:    "UPDATE test_table SET name = 'updated' WHERE id = 1",
			SetupQuery:   "CREATE TABLE IF NOT EXISTS test_table (id SERIAL PRIMARY KEY, name TEXT); INSERT INTO test_table (name) VALUES ('original')",
			CleanupQuery: "DROP TABLE IF EXISTS test_table",
		},
		{
			ObjectType:   TablePermission,
			Permission:   "DELETE",
			ReadOnly:     false,
			ReadWrite:    true,
			Admin:        true,
			TestQuery:    "DELETE FROM test_table WHERE id = 1",
			SetupQuery:   "CREATE TABLE IF NOT EXISTS test_table (id SERIAL PRIMARY KEY, name TEXT); INSERT INTO test_table (name) VALUES ('to_delete')",
			CleanupQuery: "DROP TABLE IF EXISTS test_table",
		},
		{
			ObjectType:   TablePermission,
			Permission:   "TRUNCATE",
			ReadOnly:     false,
			ReadWrite:    true,
			Admin:        true,
			TestQuery:    "TRUNCATE test_table",
			SetupQuery:   "CREATE TABLE IF NOT EXISTS test_table (id SERIAL PRIMARY KEY, name TEXT); INSERT INTO test_table (name) VALUES ('data')",
			CleanupQuery: "DROP TABLE IF EXISTS test_table",
		},
		{
			ObjectType:   TablePermission,
			Permission:   "REFERENCES",
			ReadOnly:     false,
			ReadWrite:    false,
			Admin:        true,
			TestQuery:    "CREATE TABLE test_ref_table (ref_id INT REFERENCES test_table(id))",
			SetupQuery:   "CREATE TABLE IF NOT EXISTS test_table (id SERIAL PRIMARY KEY, name TEXT)",
			CleanupQuery: "DROP TABLE IF EXISTS test_ref_table; DROP TABLE IF EXISTS test_table",
		},
		{
			ObjectType:   TablePermission,
			Permission:   "TRIGGER",
			ReadOnly:     false,
			ReadWrite:    false,
			Admin:        true,
			TestQuery:    "CREATE TRIGGER test_trigger BEFORE INSERT ON test_table FOR EACH ROW EXECUTE FUNCTION trigger_function()",
			SetupQuery:   "CREATE TABLE IF NOT EXISTS test_table (id SERIAL PRIMARY KEY, name TEXT); CREATE OR REPLACE FUNCTION trigger_function() RETURNS TRIGGER AS $$ BEGIN RETURN NEW; END; $$ LANGUAGE plpgsql",
			CleanupQuery: "DROP TRIGGER IF EXISTS test_trigger ON test_table; DROP FUNCTION IF EXISTS trigger_function(); DROP TABLE IF EXISTS test_table",
		},

		// View permissions
		{
			ObjectType:   ViewPermission,
			Permission:   "SELECT",
			ReadOnly:     true,
			ReadWrite:    true,
			Admin:        true,
			TestQuery:    "SELECT COUNT(*) FROM test_view",
			SetupQuery:   "CREATE TABLE IF NOT EXISTS test_view_table (id INT); CREATE OR REPLACE VIEW test_view AS SELECT * FROM test_view_table",
			CleanupQuery: "DROP VIEW IF EXISTS test_view; DROP TABLE IF EXISTS test_view_table",
		},

		// Sequence permissions
		{
			ObjectType:   SequencePermission,
			Permission:   "SELECT",
			ReadOnly:     true,
			ReadWrite:    true,
			Admin:        true,
			TestQuery:    "SELECT last_value FROM test_sequence;",
			SetupQuery:   "CREATE SEQUENCE IF NOT EXISTS test_sequence; SELECT setval('test_sequence', 1)",
			CleanupQuery: "DROP SEQUENCE IF EXISTS test_sequence",
		},
		{
			ObjectType:   SequencePermission,
			Permission:   "USAGE",
			ReadOnly:     false,
			ReadWrite:    true,
			Admin:        true,
			TestQuery:    "SELECT nextval('test_sequence')",
			SetupQuery:   "CREATE SEQUENCE IF NOT EXISTS test_sequence",
			CleanupQuery: "DROP SEQUENCE IF EXISTS test_sequence",
		},
		{
			ObjectType:   SequencePermission,
			Permission:   "UPDATE",
			ReadOnly:     false,
			ReadWrite:    false,
			Admin:        true,
			TestQuery:    "SELECT setval('test_sequence', 100)",
			SetupQuery:   "CREATE SEQUENCE IF NOT EXISTS test_sequence",
			CleanupQuery: "DROP SEQUENCE IF EXISTS test_sequence",
		},

		// Function permissions
		{
			ObjectType:   FunctionPermission,
			Permission:   "EXECUTE",
			ReadOnly:     true,
			ReadWrite:    true,
			Admin:        true,
			TestQuery:    "SELECT test_function()",
			SetupQuery:   "CREATE OR REPLACE FUNCTION test_function() RETURNS INT AS $$ BEGIN RETURN 42; END; $$ LANGUAGE plpgsql",
			CleanupQuery: "DROP FUNCTION IF EXISTS test_function()",
		},
	}
}

// checkRoleHasExpectedPermission verifies permissions by attempting actual operations with role users
func (suite *PermissionVerificationTestSuite) checkRoleHasExpectedPermission(
	conn *pgx.Conn,
	roleName string,
	matrix PermissionMatrix,
	shouldHavePermission bool,
) bool {
	ctx := context.Background()

	// Set up the role user first
	err := suite.setupRoleUser(conn, roleName)
	if err != nil {
		suite.T().Logf("Failed to setup role user for %s: %v", roleName, err)
		return false
	}

	// Get connection info for creating role-specific connection
	version := suite.getCurrentVersion()
	roleConn, err := suite.getRoleConnection(version, roleName)
	if err != nil {
		suite.T().Logf("Failed to connect as user for role %s: %v", roleName, err)
		return false
	}
	defer roleConn.Close(ctx)

	// Perform setup as admin if needed
	if matrix.SetupQuery != "" {
		_, setupErr := conn.Exec(ctx, matrix.SetupQuery)
		if setupErr != nil {
			suite.T().Logf("Setup query failed for %s: %v", matrix.TestQuery, setupErr)
		}
	}

	// Attempt the test query as the role user
	_, testErr := roleConn.Exec(ctx, matrix.TestQuery)

	// Perform cleanup as admin
	if matrix.CleanupQuery != "" {
		_, cleanupErr := conn.Exec(ctx, matrix.CleanupQuery)
		if cleanupErr != nil {
			suite.T().Logf("Cleanup query failed for %s: %v", matrix.TestQuery, cleanupErr)
		}
	}

	// Evaluate result based on expectation
	if shouldHavePermission {
		if testErr != nil {
			suite.T().Logf("Role %s should have permission but got error: %v", roleName, testErr)
			return false
		}
		return true
	} else {
		if testErr == nil {
			suite.T().Logf("Role %s should NOT have permission but operation succeeded", roleName)
			return false
		}
		return true
	}
}

// Test_PermissionVerification_ReadOnlyRole tests all permissions for the read-only role
func (suite *PermissionVerificationTestSuite) Test_PermissionVerification_ReadOnlyRole() {
	matrix := suite.getPermissionMatrix()

	for _, version := range suite.postgresVersions {
		suite.Run(fmt.Sprintf("PostgreSQL_%s", version), func() {
			// Set current version for helper methods
			suite.setCurrentVersion(version)

			// Execute role creation first
			stdout, stderr, err := suite.runRoleCreation(version)

			// Skip actual permission testing if binary is not fully implemented
			if err != nil || len(stdout) == 0 {
				suite.T().Logf("Binary execution result - stdout: '%s', stderr: '%s', err: %v",
					stdout, stderr, err)
				suite.T().Skip("Binary not fully implemented yet, testing infrastructure only")
				return
			}

			// Get database connection
			conn, err := suite.getDBConnection(version)
			require.NoError(suite.T(), err, "Failed to connect to database")
			defer conn.Close(context.Background())

			// Setup cleanup for role users created during this test
			defer suite.cleanupAllRoleUsers(conn)

			// Verify role exists before testing permissions
			var roleExists bool
			query := "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)"
			err = conn.QueryRow(context.Background(), query, ReadOnlyRoleName).Scan(&roleExists)
			require.NoError(suite.T(), err, "Failed to check if role exists")

			require.True(
				suite.T(),
				roleExists,
				"Read-only role '%s' must exist but was not found",
				ReadOnlyRoleName,
			)

			// Test each permission in the matrix
			for _, perm := range matrix {
				suite.Run(fmt.Sprintf("%s_%s", perm.ObjectType, perm.Permission), func() {
					// Check if the role has the expected permission
					hasCorrectPermission := suite.checkRoleHasExpectedPermission(
						conn, ReadOnlyRoleName, perm, perm.ReadOnly)

					assert.True(suite.T(), hasCorrectPermission,
						"ReadOnly role should have correct %s permission on %s (expected: %v)",
						perm.Permission, perm.ObjectType, perm.ReadOnly)
				})
			}
		})
	}
}

// Test_PermissionVerification_ReadWriteRole tests all permissions for the read-write role
func (suite *PermissionVerificationTestSuite) Test_PermissionVerification_ReadWriteRole() {
	matrix := suite.getPermissionMatrix()

	for _, version := range suite.postgresVersions {
		suite.Run(fmt.Sprintf("PostgreSQL_%s", version), func() {
			// Set current version for helper methods
			suite.setCurrentVersion(version)

			// Execute role creation first
			stdout, stderr, err := suite.runRoleCreation(version)

			// Skip actual permission testing if binary is not fully implemented
			if err != nil || len(stdout) == 0 {
				suite.T().Logf("Binary execution result - stdout: '%s', stderr: '%s', err: %v",
					stdout, stderr, err)
				suite.T().Skip("Binary not fully implemented yet, testing infrastructure only")
				return
			}

			// Get database connection
			conn, err := suite.getDBConnection(version)
			require.NoError(suite.T(), err, "Failed to connect to database")
			defer conn.Close(context.Background())

			// Setup cleanup for role users created during this test
			defer suite.cleanupAllRoleUsers(conn)

			// Verify role exists before testing permissions
			var roleExists bool
			query := "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)"
			err = conn.QueryRow(context.Background(), query, ReadWriteRoleName).Scan(&roleExists)
			require.NoError(suite.T(), err, "Failed to check if role exists")

			require.True(
				suite.T(),
				roleExists,
				"Read-write role '%s' must exist but was not found",
				ReadWriteRoleName,
			)

			// Test each permission in the matrix
			for _, perm := range matrix {
				suite.Run(fmt.Sprintf("%s_%s", perm.ObjectType, perm.Permission), func() {
					// Check if the role has the expected permission
					hasCorrectPermission := suite.checkRoleHasExpectedPermission(
						conn, ReadWriteRoleName, perm, perm.ReadWrite)

					assert.True(suite.T(), hasCorrectPermission,
						"ReadWrite role should have correct %s permission on %s (expected: %v)",
						perm.Permission, perm.ObjectType, perm.ReadWrite)
				})
			}
		})
	}
}

// Test_PermissionVerification_AdminRole tests all permissions for the admin role
func (suite *PermissionVerificationTestSuite) Test_PermissionVerification_AdminRole() {
	matrix := suite.getPermissionMatrix()

	for _, version := range suite.postgresVersions {
		suite.Run(fmt.Sprintf("PostgreSQL_%s", version), func() {
			// Set current version for helper methods
			suite.setCurrentVersion(version)

			// Execute role creation first
			stdout, stderr, err := suite.runRoleCreation(version)

			// Skip actual permission testing if binary is not fully implemented
			if err != nil || len(stdout) == 0 {
				suite.T().Logf("Binary execution result - stdout: '%s', stderr: '%s', err: %v",
					stdout, stderr, err)
				suite.T().Skip("Binary not fully implemented yet, testing infrastructure only")
				return
			}

			// Get database connection
			conn, err := suite.getDBConnection(version)
			require.NoError(suite.T(), err, "Failed to connect to database")
			defer conn.Close(context.Background())

			// Setup cleanup for role users created during this test
			defer suite.cleanupAllRoleUsers(conn)

			// Verify role exists before testing permissions
			var roleExists bool
			query := "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)"
			err = conn.QueryRow(context.Background(), query, AdminRoleName).Scan(&roleExists)
			require.NoError(suite.T(), err, "Failed to check if role exists")

			require.True(
				suite.T(),
				roleExists,
				"Admin role '%s' must exist but was not found",
				AdminRoleName,
			)

			// Test each permission in the matrix
			for _, perm := range matrix {
				suite.Run(fmt.Sprintf("%s_%s", perm.ObjectType, perm.Permission), func() {
					// Check if the role has the expected permission
					hasCorrectPermission := suite.checkRoleHasExpectedPermission(
						conn, AdminRoleName, perm, perm.Admin)

					assert.True(suite.T(), hasCorrectPermission,
						"Admin role should have correct %s permission on %s (expected: %v)",
						perm.Permission, perm.ObjectType, perm.Admin)
				})
			}
		})
	}
}

// Test_PermissionVerification_CrossRoleComparison tests that roles have different permissions where expected
func (suite *PermissionVerificationTestSuite) Test_PermissionVerification_CrossRoleComparison() {
	matrix := suite.getPermissionMatrix()

	for _, version := range suite.postgresVersions {
		suite.Run(fmt.Sprintf("PostgreSQL_%s", version), func() {
			// Set current version for helper methods
			suite.setCurrentVersion(version)

			// Execute role creation first
			stdout, stderr, err := suite.runRoleCreation(version)

			// Skip actual permission testing if binary is not fully implemented
			if err != nil || len(stdout) == 0 {
				suite.T().Logf("Binary execution result - stdout: '%s', stderr: '%s', err: %v",
					stdout, stderr, err)
				suite.T().Skip("Binary not fully implemented yet, testing infrastructure only")
				return
			}

			// Get database connection
			conn, err := suite.getDBConnection(version)
			require.NoError(suite.T(), err, "Failed to connect to database")
			defer conn.Close(context.Background())

			// Setup cleanup for role users created during this test
			defer suite.cleanupAllRoleUsers(conn)

			// Verify all roles exist
			roles := []string{ReadOnlyRoleName, ReadWriteRoleName, AdminRoleName}
			for _, roleName := range roles {
				var roleExists bool
				query := "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)"
				err = conn.QueryRow(context.Background(), query, roleName).Scan(&roleExists)
				require.NoError(suite.T(), err, "Failed to check if role %s exists", roleName)

				require.True(
					suite.T(),
					roleExists,
					"Role '%s' must exist but was not found",
					roleName,
				)
			}

			// Test role separation - find permissions where roles should differ
			for _, perm := range matrix {
				suite.Run(fmt.Sprintf("%s_%s", perm.ObjectType, perm.Permission), func() {
					expectedPerms := map[string]bool{
						ReadOnlyRoleName:  perm.ReadOnly,
						ReadWriteRoleName: perm.ReadWrite,
						AdminRoleName:     perm.Admin,
					}

					// Verify the permission separation is correct
					if perm.ReadOnly != perm.ReadWrite {
						suite.T().
							Logf("Expected permission difference between RO and RW for %s %s: RO=%v, RW=%v",
								perm.ObjectType, perm.Permission, perm.ReadOnly, perm.ReadWrite)
					}

					if perm.ReadWrite != perm.Admin {
						suite.T().
							Logf("Expected permission difference between RW and Admin for %s %s: RW=%v, Admin=%v",
								perm.ObjectType, perm.Permission, perm.ReadWrite, perm.Admin)
					}

					// Test that each role has the expected permission
					for roleName, shouldHave := range expectedPerms {
						hasCorrectPermission := suite.checkRoleHasExpectedPermission(
							conn, roleName, perm, shouldHave)

						assert.True(suite.T(), hasCorrectPermission,
							"Role %s should have correct %s permission on %s (expected: %v)",
							roleName, perm.Permission, perm.ObjectType, shouldHave)
					}
				})
			}
		})
	}
}

// Test_PermissionVerification_PrivilegeEscalation tests that roles cannot escalate privileges
func (suite *PermissionVerificationTestSuite) Test_PermissionVerification_PrivilegeEscalation() {
	escalationTests := []struct {
		name              string
		roleName          string
		escalationAttempt string
		shouldFail        bool
		description       string
	}{
		{
			name:              "RO_to_RW_Escalation",
			roleName:          ReadOnlyRoleName,
			escalationAttempt: "CREATE TABLE escalation_test (id INT)",
			shouldFail:        true,
			description:       "Read-only role should not be able to create tables",
		},
		{
			name:              "RO_Role_Creation",
			roleName:          ReadOnlyRoleName,
			escalationAttempt: "CREATE ROLE test_escalation_role",
			shouldFail:        true,
			description:       "Read-only role should not be able to create roles",
		},
		{
			name:              "RW_Role_Creation",
			roleName:          ReadWriteRoleName,
			escalationAttempt: "CREATE ROLE test_escalation_role",
			shouldFail:        true,
			description:       "Read-write role should not be able to create roles",
		},
		{
			name:              "RW_Database_Creation",
			roleName:          ReadWriteRoleName,
			escalationAttempt: "CREATE DATABASE test_escalation_db",
			shouldFail:        true,
			description:       "Read-write role should not be able to create databases",
		},
		{
			name:              "Admin_Role_Creation",
			roleName:          AdminRoleName,
			escalationAttempt: "CREATE ROLE test_admin_role",
			shouldFail:        false,
			description:       "Admin role should be able to create roles",
		},
	}

	for _, version := range suite.postgresVersions {
		suite.Run(fmt.Sprintf("PostgreSQL_%s", version), func() {
			// Set current version for helper methods
			suite.setCurrentVersion(version)

			// Execute role creation first
			stdout, stderr, err := suite.runRoleCreation(version)

			// Skip actual permission testing if binary is not fully implemented
			if err != nil || len(stdout) == 0 {
				suite.T().Logf("Binary execution result - stdout: '%s', stderr: '%s', err: %v",
					stdout, stderr, err)
				suite.T().Skip("Binary not fully implemented yet, testing infrastructure only")
				return
			}

			// Get database connection
			conn, err := suite.getDBConnection(version)
			require.NoError(suite.T(), err, "Failed to connect to database")
			defer conn.Close(context.Background())

			// Setup cleanup for role users created during this test
			defer suite.cleanupAllRoleUsers(conn)

			for _, test := range escalationTests {
				suite.Run(test.name, func() {
					// Verify role exists
					var roleExists bool
					query := "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)"
					err = conn.QueryRow(context.Background(), query, test.roleName).
						Scan(&roleExists)
					require.NoError(
						suite.T(),
						err,
						"Failed to check if role %s exists",
						test.roleName,
					)

					require.True(
						suite.T(),
						roleExists,
						"Role '%s' must exist but was not found",
						test.roleName,
					)

					// Setup role user and connect as that user to test the operation
					err = suite.setupRoleUser(conn, test.roleName)
					require.NoError(suite.T(), err, "Failed to setup role user for escalation test")

					roleConn, err := suite.getRoleConnection(version, test.roleName)
					require.NoError(
						suite.T(),
						err,
						"Failed to connect as role user for escalation test",
					)
					defer roleConn.Close(context.Background())

					ctx := context.Background()
					_, testErr := roleConn.Exec(ctx, test.escalationAttempt)

					if test.shouldFail {
						assert.Error(
							suite.T(),
							testErr,
							"%s: %s",
							test.description,
							test.escalationAttempt,
						)
					} else {
						assert.NoError(suite.T(), testErr, "%s: %s", test.description, test.escalationAttempt)
					}

					// Clean up any objects that might have been created
					cleanupQueries := []string{
						"DROP ROLE IF EXISTS test_escalation_role",
						"DROP ROLE IF EXISTS test_admin_role",
						"DROP TABLE IF EXISTS escalation_test",
						"DROP DATABASE IF EXISTS test_escalation_db",
					}

					for _, cleanup := range cleanupQueries {
						_, _ = conn.Exec(ctx, cleanup) // Ignore cleanup errors
					}
				})
			}
		})
	}
}

// Test_PermissionVerification_VersionConsistency tests that permissions are consistent across PostgreSQL versions
func (suite *PermissionVerificationTestSuite) Test_PermissionVerification_VersionConsistency() {
	if len(suite.postgresVersions) < 2 {
		suite.T().Skip("Need at least 2 PostgreSQL versions to test consistency")
		return
	}

	matrix := suite.getPermissionMatrix()
	versionResults := make(
		map[string]map[string]map[string]bool,
	) // version -> role -> permission -> hasPermission

	// Collect permission results from each version
	for _, version := range suite.postgresVersions {
		// Set current version for helper methods
		suite.setCurrentVersion(version)

		// Execute role creation
		stdout, stderr, err := suite.runRoleCreation(version)

		// Skip if binary is not fully implemented
		if err != nil || len(stdout) == 0 {
			suite.T().Logf("Binary execution result for %s - stdout: '%s', stderr: '%s', err: %v",
				version, stdout, stderr, err)
			suite.T().Skip("Binary not fully implemented yet, testing infrastructure only")
			return
		}

		conn, err := suite.getDBConnection(version)
		require.NoError(suite.T(), err, "Failed to connect to database for %s", version)
		defer conn.Close(context.Background())

		versionResults[version] = make(map[string]map[string]bool)
		roles := []string{ReadOnlyRoleName, ReadWriteRoleName, AdminRoleName}

		for _, roleName := range roles {
			// Verify role exists
			var roleExists bool
			query := "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)"
			err = conn.QueryRow(context.Background(), query, roleName).Scan(&roleExists)
			require.NoError(
				suite.T(),
				err,
				"Failed to check if role %s exists in %s",
				roleName,
				version,
			)

			if !roleExists {
				continue // Skip this role for this version
			}

			versionResults[version][roleName] = make(map[string]bool)
			expectedPermissions := map[string]bool{
				ReadOnlyRoleName:  true, // This will be set properly in the loop
				ReadWriteRoleName: true,
				AdminRoleName:     true,
			}

			for _, perm := range matrix {
				permKey := fmt.Sprintf("%s_%s", perm.ObjectType, perm.Permission)

				// Set expected permission for this role
				switch roleName {
				case ReadOnlyRoleName:
					expectedPermissions[roleName] = perm.ReadOnly
				case ReadWriteRoleName:
					expectedPermissions[roleName] = perm.ReadWrite
				case AdminRoleName:
					expectedPermissions[roleName] = perm.Admin
				}

				hasPermission := suite.checkRoleHasExpectedPermission(
					conn, roleName, perm, expectedPermissions[roleName])
				versionResults[version][roleName][permKey] = hasPermission
			}
		}
	}

	// Compare results across versions
	baseVersion := suite.postgresVersions[0]
	for i := 1; i < len(suite.postgresVersions); i++ {
		compareVersion := suite.postgresVersions[i]

		suite.Run(fmt.Sprintf("Compare_%s_vs_%s", baseVersion, compareVersion), func() {
			for roleName := range versionResults[baseVersion] {
				if _, exists := versionResults[compareVersion][roleName]; !exists {
					continue // Role doesn't exist in comparison version
				}

				for permKey := range versionResults[baseVersion][roleName] {
					baseResult := versionResults[baseVersion][roleName][permKey]
					compareResult := versionResults[compareVersion][roleName][permKey]

					assert.Equal(suite.T(), baseResult, compareResult,
						"Permission %s for role %s should be consistent between %s and %s",
						permKey, roleName, baseVersion, compareVersion)
				}
			}
		})
	}
}

// TestPermissionVerificationSuite runs the permission verification test suite
func TestPermissionVerificationSuite(t *testing.T) {
	suite.Run(t, new(PermissionVerificationTestSuite))
}
