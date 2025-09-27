package e2e

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// CLITestSuite provides test infrastructure for CLI interface testing
type CLITestSuite struct {
	suite.Suite
	binaryPath     string
	postgresC      *postgres.PostgresContainer
	connectionInfo ConnectionInfo
}

// ConnectionInfo holds database connection details
type ConnectionInfo struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

// SetupSuite builds the binary and starts test infrastructure
func (suite *CLITestSuite) SetupSuite() {
	// Build the pg-genrole binary for testing
	suite.buildBinary()

	// Start PostgreSQL container for database connection tests
	suite.startPostgresContainer()
}

// TearDownSuite cleans up test infrastructure
func (suite *CLITestSuite) TearDownSuite() {
	if suite.postgresC != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := suite.postgresC.Terminate(ctx); err != nil {
			suite.T().Logf("Warning: Failed to terminate container: %v", err)
		}
	}

	// Clean up binary
	if suite.binaryPath != "" {
		if err := os.Remove(suite.binaryPath); err != nil {
			suite.T().Logf("Warning: Failed to remove binary: %v", err)
		}
	}
}

// buildBinary compiles the pg-genrole binary for testing
func (suite *CLITestSuite) buildBinary() {
	// Create temporary binary path
	tempDir := suite.T().TempDir()
	suite.binaryPath = filepath.Join(tempDir, "pg-genrole")

	// Use more robust path handling to avoid hard-coded relative paths
	rootDir, err := filepath.Abs("../..")
	require.NoError(suite.T(), err, "Failed to get absolute path to project root")

	// Build the binary with robust path
	cmd := exec.Command("go", "build", "-o", suite.binaryPath,
		filepath.Join(rootDir, "cmd", "pg-genrole"))
	output, err := cmd.CombinedOutput()
	require.NoError(suite.T(), err, "Failed to build binary: %s", string(output))

	// Verify binary exists and is executable
	info, err := os.Stat(suite.binaryPath)
	require.NoError(suite.T(), err, "Binary not found after build")
	require.True(suite.T(), info.Mode()&0111 != 0, "Binary is not executable")
}

// startPostgresContainer initializes PostgreSQL container for database tests
func (suite *CLITestSuite) startPostgresContainer() {
	ctx := context.Background()

	// Start PostgreSQL container
	container, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(suite.T(), err, "Failed to start PostgreSQL container")

	suite.postgresC = container

	// Get connection details
	host, err := container.Host(ctx)
	require.NoError(suite.T(), err)

	port, err := container.MappedPort(ctx, "5432")
	require.NoError(suite.T(), err)

	suite.connectionInfo = ConnectionInfo{
		Host:     host,
		Port:     port.Port(),
		User:     "testuser",
		Password: "testpass",
		Database: "testdb",
	}
}

// runCommand executes the pg-genrole binary with given arguments
func (suite *CLITestSuite) runCommand(args ...string) (stdout, stderr string, err error) {
	cmd := exec.Command(suite.binaryPath, args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	return stdoutBuf.String(), stderrBuf.String(), err
}

// runCommandWithEnv executes the pg-genrole binary with environment variables
func (suite *CLITestSuite) runCommandWithEnv(
	env []string,
	args ...string,
) (stdout, stderr string, err error) {
	cmd := exec.Command(suite.binaryPath, args...)
	cmd.Env = append(os.Environ(), env...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	return stdoutBuf.String(), stderrBuf.String(), err
}

// Test_CLI_HelpCommand tests the --help command line argument
func (suite *CLITestSuite) Test_CLI_HelpCommand() {
	testCases := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name: "Short help flag",
			args: []string{"-h"},
			expected: []string{
				"pg-genrole",
				"Usage:",
				"PostgreSQL role automation tool",
				"--database",
				"--host",
				"--port",
				"--user",
				"--password",
			},
		},
		{
			name: "Long help flag",
			args: []string{"--help"},
			expected: []string{
				"pg-genrole",
				"Usage:",
				"PostgreSQL role automation tool",
				"--database",
				"--host",
				"--port",
				"--user",
				"--password",
			},
		},
		{
			name: "Help command",
			args: []string{"help"},
			expected: []string{
				"pg-genrole",
				"Usage:",
				"Commands:",
				"Options:",
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			stdout, stderr, err := suite.runCommand(tc.args...)

			// Help should exit with code 0
			assert.NoError(suite.T(), err, "Help command should exit successfully")

			// Help text should be in stdout with proper structure
			for _, expected := range tc.expected {
				assert.Contains(suite.T(), stdout, expected,
					"Help output should contain '%s'", expected)
			}

			// Verify help output has proper structure
			assert.Regexp(suite.T(),
				`(?i)usage:.*pg-genrole`,
				stdout,
				"Help should contain proper usage line")

			// Verify essential connection flags are documented
			assert.Regexp(suite.T(),
				`(?i)--(host|database|user).*description`,
				stdout,
				"Help should document connection flags with descriptions")

			// Stderr should be empty for help
			assert.Empty(suite.T(), stderr, "Help command should not output to stderr")
		})
	}
}

// Test_CLI_VersionCommand tests the --version command line argument
func (suite *CLITestSuite) Test_CLI_VersionCommand() {
	testCases := []struct {
		name string
		args []string
	}{
		{
			name: "Short version flag",
			args: []string{"-v"},
		},
		{
			name: "Long version flag",
			args: []string{"--version"},
		},
		{
			name: "Version command",
			args: []string{"version"},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			stdout, stderr, err := suite.runCommand(tc.args...)

			// Version should exit with code 0
			assert.NoError(suite.T(), err, "Version command should exit successfully")

			// Version output should contain version information with proper format
			assert.Contains(suite.T(), stdout, "pg-genrole",
				"Version output should contain program name")

			// More specific version pattern matching
			assert.Regexp(suite.T(),
				`pg-genrole\s+(version\s+)?v?\d+\.\d+\.\d+`,
				stdout,
				"Version output should contain program name and semantic version")

			// Ensure version is not just any random number
			assert.Regexp(suite.T(),
				`\b(v?\d+\.\d+\.\d+(-\w+)?)\b`,
				stdout,
				"Version should follow semantic versioning pattern")

			// Stderr should be empty for version
			assert.Empty(suite.T(), stderr, "Version command should not output to stderr")
		})
	}
}

// Test_CLI_ConnectionArguments tests database connection arguments
func (suite *CLITestSuite) Test_CLI_ConnectionArguments() {
	testCases := []struct {
		name string
		args []string
		env  []string
	}{
		{
			name: "All connection flags",
			args: []string{
				"--host", suite.connectionInfo.Host,
				"--port", suite.connectionInfo.Port,
				"--user", suite.connectionInfo.User,
				"--password", suite.connectionInfo.Password,
				"--database", suite.connectionInfo.Database,
				"--dry-run",
			},
		},
		{
			name: "Short connection flags",
			args: []string{
				"-H", suite.connectionInfo.Host,
				"-p", suite.connectionInfo.Port,
				"-u", suite.connectionInfo.User,
				"-P", suite.connectionInfo.Password,
				"-d", suite.connectionInfo.Database,
				"--dry-run",
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			var stdout, stderr string
			var err error

			if len(tc.env) > 0 {
				stdout, stderr, err = suite.runCommandWithEnv(tc.env, tc.args...)
			} else {
				stdout, stderr, err = suite.runCommand(tc.args...)
			}

			// Connection parsing should succeed in dry-run mode
			assert.NoError(suite.T(), err, "Connection arguments should be valid")

			// Make more specific assertions about connection parsing
			// Look for structured output patterns instead of generic substring matching
			assert.Regexp(suite.T(),
				`(?i)(host|server).*`+regexp.QuoteMeta(suite.connectionInfo.Host),
				stdout,
				"Output should show parsed connection host in structured format")

			assert.Regexp(suite.T(),
				`(?i)(database|db).*`+regexp.QuoteMeta(suite.connectionInfo.Database),
				stdout,
				"Output should show parsed target database in structured format")

			// Verify dry-run mode is acknowledged
			assert.Regexp(suite.T(),
				`(?i)(dry.?run|simulation|would|preview)`,
				stdout,
				"Output should indicate dry-run mode is active")

			// No errors in stderr
			assert.Empty(suite.T(), stderr, "Should not have connection errors in dry-run")
		})
	}
}

// Test_CLI_InvalidArguments tests error handling for invalid arguments
func (suite *CLITestSuite) Test_CLI_InvalidArguments() {
	testCases := []struct {
		name          string
		args          []string
		expectedError string
		errorPattern  *regexp.Regexp
	}{
		{
			name:          "Unknown flag",
			args:          []string{"--unknown-flag"},
			expectedError: "unknown flag",
			errorPattern:  regexp.MustCompile(`(?i)unknown.*(flag|option).*unknown-flag`),
		},
		{
			name:          "Invalid port number",
			args:          []string{"--port", "invalid"},
			expectedError: "invalid port",
			errorPattern:  regexp.MustCompile(`(?i)invalid.*(port|number).*invalid`),
		},
		{
			name:          "Missing required argument",
			args:          []string{"--host"},
			expectedError: "flag needs an argument",
			errorPattern: regexp.MustCompile(
				`(?i)(flag|option).*needs.*argument|missing.*argument`,
			),
		},
		{
			name:          "Empty host",
			args:          []string{"--host", ""},
			expectedError: "host cannot be empty",
			errorPattern:  regexp.MustCompile(`(?i)host.*empty|empty.*host`),
		},
		{
			name:          "Invalid port range",
			args:          []string{"--port", "70000"},
			expectedError: "port out of range",
			errorPattern:  regexp.MustCompile(`(?i)port.*(out of range|invalid range|range|70000)`),
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			stdout, stderr, err := suite.runCommand(tc.args...)

			// Should exit with error
			assert.Error(suite.T(), err, "Invalid arguments should cause error")

			// Use structured error checking with patterns
			output := stdout + stderr
			if tc.errorPattern != nil {
				assert.Regexp(suite.T(), tc.errorPattern, output,
					"Error output should match expected pattern for: %s", tc.expectedError)
			} else {
				// Fallback to substring matching if no pattern provided
				assert.Contains(suite.T(), strings.ToLower(output),
					strings.ToLower(tc.expectedError),
					"Error output should contain expected error message")
			}
		})
	}
}

// Test_CLI_DefaultDatabase tests behavior when no database is specified
func (suite *CLITestSuite) Test_CLI_DefaultDatabase() {
	suite.Run("No database specified should process all databases", func() {
		args := []string{
			"--host", suite.connectionInfo.Host,
			"--port", suite.connectionInfo.Port,
			"--user", suite.connectionInfo.User,
			"--password", suite.connectionInfo.Password,
			"--dry-run",
		}

		stdout, stderr, err := suite.runCommand(args...)

		// Should succeed and indicate all databases will be processed
		assert.NoError(suite.T(), err, "Should handle missing database gracefully")

		// Use more specific pattern matching for "all databases" indication
		assert.Regexp(suite.T(),
			`(?i)(all\s+(databases|dbs)|every\s+database|process.*all.*database)`,
			stdout,
			"Should clearly indicate all databases will be processed")

		// Verify dry-run mode is acknowledged when no specific database given
		assert.Regexp(suite.T(),
			`(?i)(dry.?run|simulation|would.*process|preview)`,
			stdout,
			"Should indicate dry-run mode when processing all databases")

		assert.Empty(suite.T(), stderr, "Should not have errors in dry-run")
	})
}

// Test_CLI_ErrorHandling tests comprehensive error handling
func (suite *CLITestSuite) Test_CLI_ErrorHandling() {
	testCases := []struct {
		name          string
		args          []string
		env           []string
		expectedError string
		errorInStderr bool
		errorPattern  *regexp.Regexp // More structured error matching
	}{
		{
			name: "Connection refused",
			args: []string{
				"--host", "localhost",
				"--port", "9999", // Non-existent port
				"--user", "testuser",
				"--password", "testpass",
				"--database", "testdb",
			},
			expectedError: "connection refused",
			errorInStderr: true,
			errorPattern:  regexp.MustCompile(`(?i)(connection.*(refused|failed)|connect.*error)`),
		},
		{
			name: "Invalid credentials",
			args: []string{
				"--host", suite.connectionInfo.Host,
				"--port", suite.connectionInfo.Port,
				"--user", "invaliduser",
				"--password", "wrongpass",
				"--database", suite.connectionInfo.Database,
			},
			expectedError: "authentication failed",
			errorInStderr: true,
			errorPattern: regexp.MustCompile(
				`(?i)(authentication.*(failed|error)|auth.*failed|login.*failed)`,
			),
		},
		{
			name: "Database does not exist",
			args: []string{
				"--host", suite.connectionInfo.Host,
				"--port", suite.connectionInfo.Port,
				"--user", suite.connectionInfo.User,
				"--password", suite.connectionInfo.Password,
				"--database", "nonexistent_db",
			},
			expectedError: "database",
			errorInStderr: true,
			errorPattern: regexp.MustCompile(
				`(?i)(database.*not.*exist|database.*found|nonexistent_db)`,
			),
		},
		{
			name: "Missing required permissions",
			args: []string{
				"--host", suite.connectionInfo.Host,
				"--port", suite.connectionInfo.Port,
				"--user", "unprivileged_user", // Would need to be created
				"--password", "testpass",
				"--database", suite.connectionInfo.Database,
			},
			expectedError: "permission",
			errorInStderr: true,
			errorPattern: regexp.MustCompile(
				`(?i)(permission.*(denied|insufficient)|access.*denied|privilege.*error)`,
			),
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			var stdout, stderr string
			var err error

			if len(tc.env) > 0 {
				stdout, stderr, err = suite.runCommandWithEnv(tc.env, tc.args...)
			} else {
				stdout, stderr, err = suite.runCommand(tc.args...)
			}

			// Should exit with error
			assert.Error(suite.T(), err, "Should fail with invalid connection")

			// Error message should be in the expected location
			var errorOutput string
			if tc.errorInStderr {
				errorOutput = stderr
			} else {
				errorOutput = stdout
			}

			// Use structured error pattern matching when available
			if tc.errorPattern != nil {
				assert.Regexp(suite.T(), tc.errorPattern, errorOutput,
					"Should match expected error pattern for: %s", tc.expectedError)
			} else {
				// Fallback to substring matching
				assert.Contains(suite.T(), strings.ToLower(errorOutput),
					strings.ToLower(tc.expectedError),
					"Should contain expected error message")
			}
		})
	}
}

// TestCLISuite runs the CLI test suite
func TestCLISuite(t *testing.T) {
	suite.Run(t, new(CLITestSuite))
}
