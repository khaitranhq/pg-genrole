// Package permissions implements the PostgreSQL permission matrix for role management
package permissions

import (
	"fmt"
	"strings"

	"github.com/khaitranhq/pg-genrole/internal/database"
)

// RoleType represents the type of database role
type RoleType string

const (
	ReadOnly  RoleType = "readonly"
	ReadWrite RoleType = "readwrite"
	Admin     RoleType = "admin"
)

// String returns the string representation of RoleType
func (r RoleType) String() string {
	return string(r)
}

// IsValid checks if the role type is valid
func (r RoleType) IsValid() bool {
	return r == ReadOnly || r == ReadWrite || r == Admin
}

// RoleConfig represents the configuration for a database role
type RoleConfig struct {
	Name        string
	Type        RoleType
	Database    string
	Description string
}

// GenerateRoleName generates a role name based on database and role type
func GenerateRoleName(database string, roleType RoleType) string {
	return fmt.Sprintf("%s_%s", database, roleType)
}

// PermissionGranter handles granting permissions to roles
type PermissionGranter struct {
	conn *database.Connection
}

// NewPermissionGranter creates a new PermissionGranter
func NewPermissionGranter(conn *database.Connection) *PermissionGranter {
	return &PermissionGranter{
		conn: conn,
	}
}

// CreateRole creates a database role with the specified configuration
func (pg *PermissionGranter) CreateRole(config RoleConfig, dryRun bool) error {
	statements := pg.generateCreateRoleStatements(config)

	for _, stmt := range statements {
		if dryRun {
			fmt.Printf("DRY RUN: %s\n", stmt)
			continue
		}

		_, err := pg.conn.Exec(stmt)
		if err != nil {
			return fmt.Errorf("failed to execute statement '%s': %w", stmt, err)
		}
	}

	return nil
}

// generateCreateRoleStatements generates SQL statements for creating a role
func (pg *PermissionGranter) generateCreateRoleStatements(config RoleConfig) []string {
	var statements []string

	// Apply security hardening: revoke CREATE privilege from PUBLIC role on public schema
	// This prevents regular users from creating objects in the public schema
	statements = append(statements, "REVOKE CREATE ON SCHEMA public FROM PUBLIC")

	// Create the role
	createRoleStmt := fmt.Sprintf("CREATE ROLE %s", config.Name)
	statements = append(statements, createRoleStmt)

	// Grant database connection
	statements = append(
		statements,
		fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s", config.Database, config.Name),
	)

	// Add role-specific permissions
	switch config.Type {
	case ReadOnly:
		statements = append(statements, pg.generateReadOnlyPermissions(config)...)
	case ReadWrite:
		statements = append(statements, pg.generateReadWritePermissions(config)...)
	case Admin:
		statements = append(statements, pg.generateAdminPermissions(config)...)
	}

	return statements
}

// generateReadOnlyPermissions generates permissions for read-only roles
func (pg *PermissionGranter) generateReadOnlyPermissions(config RoleConfig) []string {
	var statements []string

	// Grant usage on all schemas
	statements = append(statements, fmt.Sprintf("GRANT USAGE ON SCHEMA public TO %s", config.Name))

	// Grant select on all tables in public schema
	statements = append(
		statements,
		fmt.Sprintf("GRANT SELECT ON ALL TABLES IN SCHEMA public TO %s", config.Name),
	)

	// Grant select on all sequences (for currval)
	statements = append(
		statements,
		fmt.Sprintf("GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO %s", config.Name),
	)

	// Grant execute on all functions
	statements = append(
		statements,
		fmt.Sprintf("GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO %s", config.Name),
	)

	// Set default privileges for future objects
	statements = append(
		statements,
		fmt.Sprintf(
			"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO %s",
			config.Name,
		),
	)
	statements = append(
		statements,
		fmt.Sprintf(
			"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON SEQUENCES TO %s",
			config.Name,
		),
	)
	statements = append(
		statements,
		fmt.Sprintf(
			"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT EXECUTE ON FUNCTIONS TO %s",
			config.Name,
		),
	)

	return statements
}

// generateReadWritePermissions generates permissions for read-write roles
func (pg *PermissionGranter) generateReadWritePermissions(config RoleConfig) []string {
	var statements []string

	// Include all read-only permissions
	statements = append(statements, pg.generateReadOnlyPermissions(config)...)

	// Grant insert, update, delete on all tables
	statements = append(
		statements,
		fmt.Sprintf(
			"GRANT INSERT, UPDATE, DELETE, TRUNCATE ON ALL TABLES IN SCHEMA public TO %s",
			config.Name,
		),
	)

	// Grant usage on sequences (for nextval, setval)
	statements = append(
		statements,
		fmt.Sprintf("GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO %s", config.Name),
	)

	// Set default privileges for future objects
	statements = append(
		statements,
		fmt.Sprintf(
			"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT INSERT, UPDATE, DELETE, TRUNCATE ON TABLES TO %s",
			config.Name,
		),
	)
	statements = append(
		statements,
		fmt.Sprintf(
			"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE ON SEQUENCES TO %s",
			config.Name,
		),
	)

	return statements
}

// generateAdminPermissions generates permissions for admin roles
func (pg *PermissionGranter) generateAdminPermissions(config RoleConfig) []string {
	var statements []string

	// Include all read-write permissions
	statements = append(statements, pg.generateReadWritePermissions(config)...)

	// Grant create privilege on database
	statements = append(
		statements,
		fmt.Sprintf("GRANT CREATE ON DATABASE %s TO %s", config.Database, config.Name),
	)

	// Grant create on schema
	statements = append(statements, fmt.Sprintf("GRANT CREATE ON SCHEMA public TO %s", config.Name))

	// Grant all privileges on all objects
	statements = append(
		statements,
		fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO %s", config.Name),
	)
	statements = append(
		statements,
		fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO %s", config.Name),
	)
	statements = append(
		statements,
		fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA public TO %s", config.Name),
	)

	// Set default privileges for future objects
	statements = append(
		statements,
		fmt.Sprintf(
			"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO %s",
			config.Name,
		),
	)
	statements = append(
		statements,
		fmt.Sprintf(
			"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO %s",
			config.Name,
		),
	)
	statements = append(
		statements,
		fmt.Sprintf(
			"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON FUNCTIONS TO %s",
			config.Name,
		),
	)

	// Grant role creation privileges (for admin users)
	statements = append(statements, fmt.Sprintf("ALTER ROLE %s CREATEROLE", config.Name))

	return statements
}

// GrantSchemaPermissions grants permissions on a specific schema
func (pg *PermissionGranter) GrantSchemaPermissions(
	roleName string,
	roleType RoleType,
	schemaName string,
	dryRun bool,
) error {
	statements := pg.generateSchemaPermissions(roleName, roleType, schemaName)

	for _, stmt := range statements {
		if dryRun {
			fmt.Printf("DRY RUN: %s\n", stmt)
			continue
		}

		_, err := pg.conn.Exec(stmt)
		if err != nil {
			return fmt.Errorf("failed to execute statement '%s': %w", stmt, err)
		}
	}

	return nil
}

// generateSchemaPermissions generates schema-specific permission statements
func (pg *PermissionGranter) generateSchemaPermissions(
	roleName string,
	roleType RoleType,
	schemaName string,
) []string {
	var statements []string

	// Always grant usage on schema
	statements = append(
		statements,
		fmt.Sprintf("GRANT USAGE ON SCHEMA %s TO %s", schemaName, roleName),
	)

	switch roleType {
	case ReadOnly:
		statements = append(
			statements,
			fmt.Sprintf("GRANT SELECT ON ALL TABLES IN SCHEMA %s TO %s", schemaName, roleName),
		)
		statements = append(
			statements,
			fmt.Sprintf("GRANT SELECT ON ALL SEQUENCES IN SCHEMA %s TO %s", schemaName, roleName),
		)
		statements = append(
			statements,
			fmt.Sprintf("GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA %s TO %s", schemaName, roleName),
		)

		// Default privileges
		statements = append(
			statements,
			fmt.Sprintf(
				"ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT SELECT ON TABLES TO %s",
				schemaName,
				roleName,
			),
		)
		statements = append(
			statements,
			fmt.Sprintf(
				"ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT SELECT ON SEQUENCES TO %s",
				schemaName,
				roleName,
			),
		)
		statements = append(
			statements,
			fmt.Sprintf(
				"ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT EXECUTE ON FUNCTIONS TO %s",
				schemaName,
				roleName,
			),
		)

	case ReadWrite:
		// Include read-only permissions
		statements = append(
			statements,
			fmt.Sprintf("GRANT SELECT ON ALL TABLES IN SCHEMA %s TO %s", schemaName, roleName),
		)
		statements = append(
			statements,
			fmt.Sprintf("GRANT SELECT ON ALL SEQUENCES IN SCHEMA %s TO %s", schemaName, roleName),
		)
		statements = append(
			statements,
			fmt.Sprintf("GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA %s TO %s", schemaName, roleName),
		)

		// Add write permissions
		statements = append(
			statements,
			fmt.Sprintf(
				"GRANT INSERT, UPDATE, DELETE, TRUNCATE ON ALL TABLES IN SCHEMA %s TO %s",
				schemaName,
				roleName,
			),
		)
		statements = append(
			statements,
			fmt.Sprintf("GRANT USAGE ON ALL SEQUENCES IN SCHEMA %s TO %s", schemaName, roleName),
		)

		// Default privileges
		statements = append(
			statements,
			fmt.Sprintf(
				"ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON TABLES TO %s",
				schemaName,
				roleName,
			),
		)
		statements = append(
			statements,
			fmt.Sprintf(
				"ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT USAGE, SELECT ON SEQUENCES TO %s",
				schemaName,
				roleName,
			),
		)
		statements = append(
			statements,
			fmt.Sprintf(
				"ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT EXECUTE ON FUNCTIONS TO %s",
				schemaName,
				roleName,
			),
		)

	case Admin:
		// Grant create on schema
		statements = append(
			statements,
			fmt.Sprintf("GRANT CREATE ON SCHEMA %s TO %s", schemaName, roleName),
		)

		// Grant all privileges
		statements = append(
			statements,
			fmt.Sprintf(
				"GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA %s TO %s",
				schemaName,
				roleName,
			),
		)
		statements = append(
			statements,
			fmt.Sprintf(
				"GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA %s TO %s",
				schemaName,
				roleName,
			),
		)
		statements = append(
			statements,
			fmt.Sprintf(
				"GRANT ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA %s TO %s",
				schemaName,
				roleName,
			),
		)

		// Default privileges
		statements = append(
			statements,
			fmt.Sprintf(
				"ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT ALL ON TABLES TO %s",
				schemaName,
				roleName,
			),
		)
		statements = append(
			statements,
			fmt.Sprintf(
				"ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT ALL ON SEQUENCES TO %s",
				schemaName,
				roleName,
			),
		)
		statements = append(
			statements,
			fmt.Sprintf(
				"ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT ALL ON FUNCTIONS TO %s",
				schemaName,
				roleName,
			),
		)
	}

	return statements
}

// ValidateRolePermissions validates that a role has the expected permissions
func (pg *PermissionGranter) ValidateRolePermissions(roleName string, expectedType RoleType) error {
	// Check if role exists
	exists, err := pg.conn.CheckRoleExists(roleName)
	if err != nil {
		return fmt.Errorf("failed to check role existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("role %s does not exist", roleName)
	}

	// Additional validation logic would go here
	// For now, we just check existence
	return nil
}

// ParseRoleType parses a string into a RoleType
func ParseRoleType(s string) (RoleType, error) {
	roleType := RoleType(strings.ToLower(s))
	if !roleType.IsValid() {
		return "", fmt.Errorf("invalid role type: %s (must be one of: readonly, readwrite, admin)", s)
	}
	return roleType, nil
}
