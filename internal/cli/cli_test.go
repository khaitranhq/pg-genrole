package cli

import (
	"os"
	"testing"

	"github.com/khaitranhq/pg-genrole/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseArgs(t *testing.T) {
	// Save original args
	originalArgs := os.Args

	tests := []struct {
		name        string
		args        []string
		expectError bool
		checkFunc   func(t *testing.T, args *Args)
	}{
		{
			name: "help flag",
			args: []string{"pg-genrole", "--help"},
			checkFunc: func(t *testing.T, args *Args) {
				assert.True(t, args.ShowHelp)
			},
		},
		{
			name: "help short flag",
			args: []string{"pg-genrole", "-h"},
			checkFunc: func(t *testing.T, args *Args) {
				assert.True(t, args.ShowHelp)
			},
		},
		{
			name: "version flag",
			args: []string{"pg-genrole", "--version"},
			checkFunc: func(t *testing.T, args *Args) {
				assert.True(t, args.ShowVersion)
			},
		},
		{
			name: "version short flag",
			args: []string{"pg-genrole", "-v"},
			checkFunc: func(t *testing.T, args *Args) {
				assert.True(t, args.ShowVersion)
			},
		},
		{
			name: "complete valid config with long flags",
			args: []string{
				"pg-genrole",
				"--host", "localhost",
				"--port", "5432",
				"--user", "testuser",
				"--password", "testpass",
				"--database", "testdb",
				"--dry-run",
			},
			checkFunc: func(t *testing.T, args *Args) {
				assert.Equal(t, "localhost", args.Config.Host)
				assert.Equal(t, 5432, args.Config.Port)
				assert.Equal(t, "testuser", args.Config.User)
				assert.Equal(t, "testpass", args.Config.Password)
				assert.Equal(t, "testdb", args.Config.Database)
				assert.True(t, args.Config.DryRun)
			},
		},
		{
			name: "complete valid config with short flags",
			args: []string{
				"pg-genrole",
				"-H", "db.example.com",
				"-p", "3306",
				"-u", "admin",
				"-P", "secret",
				"-d", "myapp",
			},
			checkFunc: func(t *testing.T, args *Args) {
				assert.Equal(t, "db.example.com", args.Config.Host)
				assert.Equal(t, 3306, args.Config.Port)
				assert.Equal(t, "admin", args.Config.User)
				assert.Equal(t, "secret", args.Config.Password)
				assert.Equal(t, "myapp", args.Config.Database)
				assert.False(t, args.Config.DryRun)
			},
		},
		{
			name: "missing required host",
			args: []string{
				"pg-genrole",
				"--port", "5432",
				"--user", "testuser",
				"--password", "testpass",
			},
			expectError: true,
		},
		{
			name: "missing required user",
			args: []string{
				"pg-genrole",
				"--host", "localhost",
				"--password", "testpass",
			},
			expectError: true,
		},
		{
			name: "missing required password",
			args: []string{
				"pg-genrole",
				"--host", "localhost",
				"--user", "testuser",
			},
			expectError: true,
		},
		{
			name: "invalid port",
			args: []string{
				"pg-genrole",
				"--host", "localhost",
				"--port", "invalid",
				"--user", "testuser",
				"--password", "testpass",
			},
			expectError: true,
		},
		{
			name: "command help",
			args: []string{"pg-genrole", "help"},
			checkFunc: func(t *testing.T, args *Args) {
				assert.True(t, args.ShowHelp)
			},
		},
		{
			name: "version subcommand",
			args: []string{"pg-genrole", "version"},
			checkFunc: func(t *testing.T, args *Args) {
				// For version subcommand, we expect ShowVersion to be true
				assert.False(t, args.ShowHelp)
				assert.True(t, args.ShowVersion)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set test args
			os.Args = tt.args

			// Reset Cobra and global state
			resetCobraState()

			args, err := ParseArgs()

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, args)
				if tt.checkFunc != nil {
					tt.checkFunc(t, args)
				}
			}
		})
	}

	// Restore original args
	os.Args = originalArgs
}

// TestExecuteDirectly tests calling Execute() directly
func TestExecute(t *testing.T) {
	// Save original args
	originalArgs := os.Args

	t.Run("valid arguments", func(t *testing.T) {
		os.Args = []string{
			"pg-genrole",
			"--host", "localhost",
			"--user", "testuser",
			"--password", "testpass",
		}

		resetCobraState()

		err := Execute()
		assert.NoError(t, err)
	})

	t.Run("missing required flag", func(t *testing.T) {
		os.Args = []string{
			"pg-genrole",
			"--host", "localhost",
		}

		resetCobraState()

		err := Execute()
		assert.Error(t, err)
	})

	// Restore original args
	os.Args = originalArgs
}

// TestVersionCommand tests the version subcommand specifically
func TestVersionCommand(t *testing.T) {
	originalArgs := os.Args

	t.Run("version subcommand", func(t *testing.T) {
		os.Args = []string{"pg-genrole", "version"}

		resetCobraState()

		// This should not error - it should just print version info
		err := Execute()
		assert.NoError(t, err)
	})

	os.Args = originalArgs
}

// resetCobraState resets Cobra's internal state for testing
// This is necessary because Cobra's root command needs to be reset between tests
func resetCobraState() {
	// Reset global variables
	globalArgs = nil
	showHelp = false
	showVer = false

	// Reset config
	cfg = config.Config{}

	// Reset Cobra command flags to default values
	// Note: We need to reset the flag values manually
	cfg.Host = ""
	cfg.Port = 5432
	cfg.User = ""
	cfg.Password = ""
	cfg.Database = ""
	cfg.DryRun = false
}
