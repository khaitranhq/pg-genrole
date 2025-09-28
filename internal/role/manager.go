// Package role manages PostgreSQL database role creation and lifecycle
package role

import (
	"fmt"
	"slices"
	"strings"

	"github.com/khaitranhq/pg-genrole/internal/database"
	"github.com/khaitranhq/pg-genrole/internal/permissions"
)

// Manager handles role management operations
type Manager struct {
	conn    *database.Connection
	granter *permissions.PermissionGranter
}

// NewManager creates a new role Manager
func NewManager(conn *database.Connection) *Manager {
	return &Manager{
		conn:    conn,
		granter: permissions.NewPermissionGranter(conn),
	}
}

// CreateDatabaseRoles creates all three role types for a database
func (m *Manager) CreateDatabaseRoles(databaseName string, dryRun bool) error {
	roleTypes := []permissions.RoleType{
		permissions.ReadOnly,
		permissions.ReadWrite,
		permissions.Admin,
	}

	fmt.Printf("Creating roles for database: %s\n", databaseName)

	for _, roleType := range roleTypes {
		roleName := permissions.GenerateRoleName(databaseName, roleType)

		// Check if role already exists
		exists, err := m.conn.CheckRoleExists(roleName)
		if err != nil {
			return fmt.Errorf("failed to check if role exists: %w", err)
		}

		if exists {
			fmt.Printf("Role %s already exists, skipping creation\n", roleName)
			continue
		}

		config := permissions.RoleConfig{
			Name:        roleName,
			Type:        roleType,
			Database:    databaseName,
			Description: fmt.Sprintf("%s role for database %s", roleType, databaseName),
		}

		fmt.Printf("Creating %s role: %s\n", roleType, roleName)

		if err := m.granter.CreateRole(config, dryRun); err != nil {
			return fmt.Errorf("failed to create role %s: %w", roleName, err)
		}

		// Grant permissions on existing schemas
		schemas, err := m.conn.GetSchemas()
		if err != nil {
			return fmt.Errorf("failed to get schemas for database %s: %w", databaseName, err)
		}

		for _, schema := range schemas {
			if schema == "public" {
				// Public schema permissions are handled in the main role creation
				continue
			}

			fmt.Printf("Granting %s permissions on schema: %s\n", roleType, schema)
			if err := m.granter.GrantSchemaPermissions(roleName, roleType, schema, dryRun); err != nil {
				return fmt.Errorf("failed to grant schema permissions on %s: %w", schema, err)
			}
		}
	}

	return nil
}

// CreateAllDatabaseRoles creates roles for all accessible databases
func (m *Manager) CreateAllDatabaseRoles(currentDatabase string, dryRun bool) error {
	databases, err := m.conn.GetDatabases()
	if err != nil {
		return fmt.Errorf("failed to get database list: %w", err)
	}

	if slices.Contains(databases, currentDatabase) && currentDatabase != "" &&
		currentDatabase != "postgres" {
		databases = append(databases, currentDatabase)
	}

	if len(databases) == 0 {
		return fmt.Errorf("no databases found to process")
	}

	fmt.Printf("Processing %d database(s): %v\n", len(databases), databases)

	for _, dbName := range databases {
		fmt.Printf("\n--- Processing database: %s ---\n", dbName)

		// For databases other than the current one, we need a new connection
		if dbName != currentDatabase {
			if err := m.processRemoteDatabase(dbName, dryRun); err != nil {
				fmt.Printf("Warning: failed to process database %s: %v\n", dbName, err)
				continue
			}
		} else {
			if err := m.CreateDatabaseRoles(dbName, dryRun); err != nil {
				return fmt.Errorf("failed to create roles for database %s: %w", dbName, err)
			}
		}
	}

	return nil
}

// processRemoteDatabase creates a connection to a specific database and creates roles
func (m *Manager) processRemoteDatabase(databaseName string, dryRun bool) error {
	// Note: In practice, you might want to create a new connection to the specific database
	// For now, we'll create roles from the current connection, which should work for most cases
	// but might not handle database-specific schemas properly

	fmt.Printf("Processing database %s from current connection\n", databaseName)
	return m.CreateDatabaseRoles(databaseName, dryRun)
}

// DropRole drops a database role
func (m *Manager) DropRole(roleName string, dryRun bool) error {
	exists, err := m.conn.CheckRoleExists(roleName)
	if err != nil {
		return fmt.Errorf("failed to check if role exists: %w", err)
	}

	if !exists {
		fmt.Printf("Role %s does not exist, nothing to drop\n", roleName)
		return nil
	}

	dropStmt := fmt.Sprintf("DROP ROLE %s", roleName)

	if dryRun {
		fmt.Printf("DRY RUN: %s\n", dropStmt)
		return nil
	}

	_, err = m.conn.Exec(dropStmt)
	if err != nil {
		return fmt.Errorf("failed to drop role %s: %w", roleName, err)
	}

	fmt.Printf("Successfully dropped role: %s\n", roleName)
	return nil
}

// ListRoles lists all roles matching the pg-genrole naming pattern
func (m *Manager) ListRoles() error {
	query := `
		SELECT rolname, rolcreatedb, rolcreaterole, rolcanlogin
		FROM pg_roles 
		WHERE rolname ~ '^.+_(readonly|readwrite|admin|ro|rw)$'
		ORDER BY rolname`

	rows, err := m.conn.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query roles: %w", err)
	}
	defer rows.Close()

	fmt.Printf("%-30s %-10s %-12s %-8s\n", "Role Name", "Type", "Create DB", "Login")
	fmt.Printf("%-30s %-10s %-12s %-8s\n",
		"-----------------------------",
		"---------",
		"-----------",
		"-------")

	for rows.Next() {
		var roleName string
		var createDB, createRole, canLogin bool

		if err := rows.Scan(&roleName, &createDB, &createRole, &canLogin); err != nil {
			return fmt.Errorf("failed to scan role: %w", err)
		}

		// Extract role type from name
		roleType := "unknown"
		if strings.HasSuffix(roleName, "_readonly") {
			roleType = "readonly"
		} else if strings.HasSuffix(roleName, "_readwrite") {
			roleType = "readwrite"
		} else if strings.HasSuffix(roleName, "_admin") {
			roleType = "admin"
		} else if strings.HasSuffix(roleName, "_ro") {
			roleType = "readonly"
		} else if strings.HasSuffix(roleName, "_rw") {
			roleType = "readwrite"
		}

		fmt.Printf("%-30s %-10s %-12t %-8t\n",
			roleName,
			roleType,
			createDB,
			canLogin)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating role rows: %w", err)
	}

	return nil
}

// ValidateRoles validates that roles have the correct permissions
func (m *Manager) ValidateRoles(databaseName string) error {
	roleTypes := []permissions.RoleType{
		permissions.ReadOnly,
		permissions.ReadWrite,
		permissions.Admin,
	}

	fmt.Printf("Validating roles for database: %s\n", databaseName)

	for _, roleType := range roleTypes {
		roleName := permissions.GenerateRoleName(databaseName, roleType)

		if err := m.granter.ValidateRolePermissions(roleName, roleType); err != nil {
			return fmt.Errorf("validation failed for role %s: %w", roleName, err)
		}

		fmt.Printf("✓ Role %s validation passed\n", roleName)
	}

	return nil
}

// GetRoleInfo returns information about a specific role
func (m *Manager) GetRoleInfo(roleName string) (*RoleInfo, error) {
	query := `
		SELECT rolname, rolcanlogin, rolcreatedb, rolcreaterole, rolsuper
		FROM pg_roles 
		WHERE rolname = $1`

	var info RoleInfo
	err := m.conn.QueryRow(query, roleName).Scan(
		&info.Name,
		&info.CanLogin,
		&info.CanCreateDB,
		&info.CanCreateRole,
		&info.IsSuperuser,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get role info for %s: %w", roleName, err)
	}

	return &info, nil
}

// RoleInfo contains information about a database role
type RoleInfo struct {
	Name          string
	CanLogin      bool
	CanCreateDB   bool
	CanCreateRole bool
	IsSuperuser   bool
}

// String returns a string representation of the role info
func (r *RoleInfo) String() string {
	return fmt.Sprintf("Role: %s, Login: %t, CreateDB: %t, CreateRole: %t, Superuser: %t",
		r.Name, r.CanLogin, r.CanCreateDB, r.CanCreateRole, r.IsSuperuser)
}
