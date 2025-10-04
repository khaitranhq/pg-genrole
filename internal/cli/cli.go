// Package cli provides command-line interface functionality for pg-genrole
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/khaitranhq/pg-genrole/internal/config"
)

const (
	version = "1.0.0"
)

// Args holds parsed command line arguments
type Args struct {
	Config config.Config

	// Command flags
	ShowHelp    bool
	ShowVersion bool
}

var (
	cfg        config.Config
	showHelp   bool
	showVer    bool
	globalArgs *Args
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "pg-genrole",
	Short: "PostgreSQL Role Generation Tool",
	Long: `pg-genrole - PostgreSQL Role Generation Tool

Automates creation of Read-Only, Read-Write, and Admin roles for PostgreSQL databases.

ROLE TYPES CREATED:
    - <database>_readonly    : Read-only access (SELECT permissions)
    - <database>_readwrite    : Read-write access (SELECT, INSERT, UPDATE, DELETE)
    - <database>_admin : Administrative access (all permissions)

PostgreSQL Version Compatibility: >= 13`,
	Example: `  # Process all databases (dry run)
  pg-genrole --host localhost --port 5432 --user admin --password secret --dry-run

  # Process specific database
  pg-genrole --host db.example.com --user admin --password secret --database myapp

  # Using short flags
  pg-genrole -H localhost -p 5432 -u admin -P secret -d myapp`,
	SilenceUsage: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Perform custom validation before Cobra's required flag validation
		// This allows us to provide more specific error messages

		// Check for empty host specifically
		if cmd.Flag("host").Changed && cfg.Host == "" {
			return fmt.Errorf("host cannot be empty")
		}

		// Check for invalid port range specifically
		if cmd.Flag("port").Changed {
			if cfg.Port <= 0 || cfg.Port > 65535 {
				return fmt.Errorf("port out of range: %d (must be 1-65535)", cfg.Port)
			}
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Set defaults and validate configuration
		cfg.SetDefaults()

		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("configuration validation failed: %w", err)
		}

		// Main application logic would go here
		// For now, just set the parsed config
		globalArgs = &Args{
			Config:      cfg,
			ShowHelp:    showHelp,
			ShowVersion: showVer,
		}
		return nil
	},
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  "Show version information for pg-genrole",
	Run: func(cmd *cobra.Command, args []string) {
		ShowVersion()
		// Set the global args to indicate this was a version command
		globalArgs = &Args{
			ShowVersion: true,
		}
	},
}

func init() {
	// Add version subcommand
	rootCmd.AddCommand(versionCmd)

	// Define global flags with both long and short versions
	rootCmd.PersistentFlags().StringVarP(&cfg.Host, "host", "H", "", "PostgreSQL server hostname or IP address (required)")
	rootCmd.PersistentFlags().IntVarP(&cfg.Port, "port", "p", 5432, "PostgreSQL server port")
	rootCmd.PersistentFlags().StringVarP(&cfg.User, "user", "u", "", "PostgreSQL username (required)")
	rootCmd.PersistentFlags().StringVarP(&cfg.Password, "password", "P", "", "PostgreSQL password (required)")
	rootCmd.PersistentFlags().StringVarP(&cfg.Database, "database", "d", "", "Specific database to process (optional, processes all if not specified)")
	rootCmd.PersistentFlags().BoolVar(&cfg.DryRun, "dry-run", false, "Show what would be done without making changes")

	// Mark required flags
	rootCmd.MarkPersistentFlagRequired("host")
	rootCmd.MarkPersistentFlagRequired("user")
	rootCmd.MarkPersistentFlagRequired("password")

	// Set custom help template to match original format
	rootCmd.SetHelpTemplate(getCustomHelpTemplate())
}

// Execute executes the root command and returns any error
func Execute() error {
	return rootCmd.Execute()
}

// ParseArgs parses command line arguments and returns configuration
// This function maintains backward compatibility with the original interface
func ParseArgs() (*Args, error) {
	// Reset global state
	globalArgs = nil
	showHelp = false
	showVer = false

	// Check for help/version flags manually before executing
	args := os.Args[1:]
	for _, arg := range args {
		switch arg {
		case "-h", "--help", "help":
			showHelp = true
			return &Args{ShowHelp: true}, nil
		case "-v", "--version", "version":
			showVer = true
			return &Args{ShowVersion: true}, nil
		}
	}

	// Execute the command
	if err := Execute(); err != nil {
		return nil, err
	}

	// Return the parsed args if execution was successful
	if globalArgs == nil {
		return &Args{}, nil
	}

	return globalArgs, nil
}

// ShowHelp prints the help message
func ShowHelp() {
	rootCmd.Help()
}

// ShowVersion prints version information
func ShowVersion() {
	fmt.Printf("pg-genrole version %s\n", version)
	fmt.Println("PostgreSQL Role Generation Tool")
	fmt.Println("Compatible with PostgreSQL 13+")
	fmt.Println()
	fmt.Println("Copyright (c) 2024")
	fmt.Println("Licensed under MIT License")
}

// getCustomHelpTemplate returns a custom help template that matches the original format
func getCustomHelpTemplate() string {
	return `{{.Long}}

{{if .HasAvailableSubCommands}}COMMANDS:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}

{{if .HasAvailableLocalFlags}}OPTIONS:
{{.LocalFlags.FlagUsages}}{{end}}

{{if .HasAvailableInheritedFlags}}GLOBAL OPTIONS:
{{.InheritedFlags.FlagUsages}}{{end}}

{{if .HasExample}}EXAMPLES:
{{.Example}}{{end}}

{{if .HasAvailableSubCommands}}Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
}
