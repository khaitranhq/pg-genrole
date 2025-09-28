package permissions

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoleType_String(t *testing.T) {
	tests := []struct {
		roleType RoleType
		expected string
	}{
		{ReadOnly, "ro"},
		{ReadWrite, "rw"},
		{Admin, "admin"},
	}

	for _, tt := range tests {
		t.Run(string(tt.roleType), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.roleType.String())
		})
	}
}

func TestRoleType_IsValid(t *testing.T) {
	tests := []struct {
		roleType RoleType
		expected bool
	}{
		{ReadOnly, true},
		{ReadWrite, true},
		{Admin, true},
		{RoleType("invalid"), false},
		{RoleType(""), false},
		{RoleType("readonly"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.roleType), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.roleType.IsValid())
		})
	}
}

func TestGenerateRoleName(t *testing.T) {
	tests := []struct {
		name     string
		database string
		roleType RoleType
		expected string
	}{
		{
			name:     "readonly role",
			database: "myapp",
			roleType: ReadOnly,
			expected: "myapp_ro",
		},
		{
			name:     "readwrite role",
			database: "testdb",
			roleType: ReadWrite,
			expected: "testdb_rw",
		},
		{
			name:     "admin role",
			database: "production",
			roleType: Admin,
			expected: "production_admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateRoleName(tt.database, tt.roleType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseRoleType(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    RoleType
		expectError bool
	}{
		{
			name:     "valid ro",
			input:    "ro",
			expected: ReadOnly,
		},
		{
			name:     "valid RO uppercase",
			input:    "RO",
			expected: ReadOnly,
		},
		{
			name:     "valid rw",
			input:    "rw",
			expected: ReadWrite,
		},
		{
			name:     "valid RW uppercase",
			input:    "RW",
			expected: ReadWrite,
		},
		{
			name:     "valid admin",
			input:    "admin",
			expected: Admin,
		},
		{
			name:     "valid ADMIN uppercase",
			input:    "ADMIN",
			expected: Admin,
		},
		{
			name:        "invalid role type",
			input:       "invalid",
			expectError: true,
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseRoleType(tt.input)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid role type")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestPermissionGranter_generateCreateRoleStatements(t *testing.T) {
	granter := &PermissionGranter{
		// We don't need a real connection for this test
		conn: nil,
	}

	tests := []struct {
		name     string
		config   RoleConfig
		contains []string // Statements that should be present
	}{
		{
			name: "readonly role",
			config: RoleConfig{
				Name:     "testdb_ro",
				Type:     ReadOnly,
				Database: "testdb",
			},
			contains: []string{
				"CREATE ROLE testdb_ro",
				"GRANT CONNECT ON DATABASE testdb TO testdb_ro",
				"GRANT USAGE ON SCHEMA public TO testdb_ro",
				"GRANT SELECT ON ALL TABLES IN SCHEMA public TO testdb_ro",
			},
		},
		{
			name: "readwrite role",
			config: RoleConfig{
				Name:     "testdb_rw",
				Type:     ReadWrite,
				Database: "testdb",
			},
			contains: []string{
				"CREATE ROLE testdb_rw",
				"GRANT CONNECT ON DATABASE testdb TO testdb_rw",
				"GRANT TEMPORARY ON DATABASE testdb TO testdb_rw",
				"GRANT INSERT, UPDATE, DELETE, TRUNCATE ON ALL TABLES IN SCHEMA public TO testdb_rw",
			},
		},
		{
			name: "admin role",
			config: RoleConfig{
				Name:     "testdb_admin",
				Type:     Admin,
				Database: "testdb",
			},
			contains: []string{
				"CREATE ROLE testdb_admin",
				"GRANT CREATE ON DATABASE testdb TO testdb_admin",
				"ALTER ROLE testdb_admin CREATEROLE",
				"GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO testdb_admin",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statements := granter.generateCreateRoleStatements(tt.config)

			// Check that all required statements are present
			for _, expectedStmt := range tt.contains {
				assert.True(t, slices.Contains(statements, expectedStmt), "Expected statement not found: %s", expectedStmt)
			}
		})
	}
}

func TestPermissionGranter_generateSchemaPermissions(t *testing.T) {
	granter := &PermissionGranter{
		// We don't need a real connection for this test
		conn: nil,
	}

	tests := []struct {
		name       string
		roleName   string
		roleType   RoleType
		schemaName string
		contains   []string
	}{
		{
			name:       "readonly schema permissions",
			roleName:   "testdb_ro",
			roleType:   ReadOnly,
			schemaName: "app_schema",
			contains: []string{
				"GRANT USAGE ON SCHEMA app_schema TO testdb_ro",
				"GRANT SELECT ON ALL TABLES IN SCHEMA app_schema TO testdb_ro",
			},
		},
		{
			name:       "readwrite schema permissions",
			roleName:   "testdb_rw",
			roleType:   ReadWrite,
			schemaName: "app_schema",
			contains: []string{
				"GRANT USAGE ON SCHEMA app_schema TO testdb_rw",
				"GRANT INSERT, UPDATE, DELETE, TRUNCATE ON ALL TABLES IN SCHEMA app_schema TO testdb_rw",
			},
		},
		{
			name:       "admin schema permissions",
			roleName:   "testdb_admin",
			roleType:   Admin,
			schemaName: "app_schema",
			contains: []string{
				"GRANT CREATE ON SCHEMA app_schema TO testdb_admin",
				"GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA app_schema TO testdb_admin",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statements := granter.generateSchemaPermissions(tt.roleName, tt.roleType, tt.schemaName)

			// Check that all required statements are present
			for _, expectedStmt := range tt.contains {
				assert.True(t, slices.Contains(statements, expectedStmt), "Expected statement not found: %s", expectedStmt)
			}
		})
	}
}
