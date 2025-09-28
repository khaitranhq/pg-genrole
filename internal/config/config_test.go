package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: Config{
				Host:     "localhost",
				Port:     5432,
				User:     "testuser",
				Password: "testpass",
				Database: "testdb",
			},
			expectError: false,
		},
		{
			name: "missing host",
			config: Config{
				Port:     5432,
				User:     "testuser",
				Password: "testpass",
			},
			expectError: true,
			errorMsg:    "Host",
		},
		{
			name: "missing user",
			config: Config{
				Host:     "localhost",
				Port:     5432,
				Password: "testpass",
			},
			expectError: true,
			errorMsg:    "User",
		},
		{
			name: "missing password",
			config: Config{
				Host: "localhost",
				Port: 5432,
				User: "testuser",
			},
			expectError: true,
			errorMsg:    "Password",
		},
		{
			name: "invalid port - zero",
			config: Config{
				Host:     "localhost",
				Port:     0,
				User:     "testuser",
				Password: "testpass",
			},
			expectError: true,
			errorMsg:    "Port",
		},
		{
			name: "invalid port - negative",
			config: Config{
				Host:     "localhost",
				Port:     -1,
				User:     "testuser",
				Password: "testpass",
			},
			expectError: true,
			errorMsg:    "Port",
		},
		{
			name: "invalid port - too high",
			config: Config{
				Host:     "localhost",
				Port:     65536,
				User:     "testuser",
				Password: "testpass",
			},
			expectError: true,
			errorMsg:    "Port",
		},
		{
			name: "valid config with minimum port",
			config: Config{
				Host:     "localhost",
				Port:     1,
				User:     "testuser",
				Password: "testpass",
			},
			expectError: false,
		},
		{
			name: "valid config with maximum port",
			config: Config{
				Host:     "localhost",
				Port:     65535,
				User:     "testuser",
				Password: "testpass",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfig_DatabaseURL(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name: "with database specified",
			config: Config{
				Host:     "localhost",
				Port:     5432,
				User:     "testuser",
				Password: "testpass",
				Database: "testdb",
			},
			expected: "postgres://testuser:testpass@localhost:5432/testdb?sslmode=prefer",
		},
		{
			name: "without database specified",
			config: Config{
				Host:     "localhost",
				Port:     5432,
				User:     "testuser",
				Password: "testpass",
			},
			expected: "postgres://testuser:testpass@localhost:5432/postgres?sslmode=prefer",
		},
		{
			name: "different host and port",
			config: Config{
				Host:     "db.example.com",
				Port:     3306,
				User:     "admin",
				Password: "secret",
				Database: "myapp",
			},
			expected: "postgres://admin:secret@db.example.com:3306/myapp?sslmode=prefer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.DatabaseURL()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_SetDefaults(t *testing.T) {
	config := Config{
		Host:     "localhost",
		User:     "testuser",
		Password: "testpass",
		// Port not set
	}

	config.SetDefaults()
	assert.Equal(t, 5432, config.Port)
}

func TestConfig_String(t *testing.T) {
	config := Config{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "secret", // Should not appear in string representation
		Database: "testdb",
		DryRun:   true,
	}

	result := config.String()
	assert.Contains(t, result, "localhost")
	assert.Contains(t, result, "5432")
	assert.Contains(t, result, "testuser")
	assert.Contains(t, result, "testdb")
	assert.Contains(t, result, "true")
	assert.NotContains(t, result, "secret") // Password should not be in string
}

func TestParsePort(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    int
		expectError bool
	}{
		{
			name:     "empty string returns default",
			input:    "",
			expected: 5432,
		},
		{
			name:     "valid port",
			input:    "3306",
			expected: 3306,
		},
		{
			name:        "invalid port string",
			input:       "not_a_number",
			expectError: true,
		},
		{
			name:        "port too low",
			input:       "0",
			expectError: true,
		},
		{
			name:        "port too high",
			input:       "65536",
			expectError: true,
		},
		{
			name:        "negative port",
			input:       "-1",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParsePort(tt.input)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
