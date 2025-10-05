package main

import (
	"context"
	"fmt"
	"os"

	"github.com/khaitranhq/pg-genrole/internal/cli"
	"github.com/khaitranhq/pg-genrole/internal/database"
	"github.com/khaitranhq/pg-genrole/internal/role"
)

func main() {
	// Parse command line arguments
	args, err := cli.ParseArgs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Use --help for usage information\n")
		os.Exit(1)
	}

	// Handle help and version flags
	if args.ShowHelp {
		cli.ShowHelp()
		return
	}

	if args.ShowVersion {
		cli.ShowVersion()
		return
	}

	// Display connection information when in dry-run mode
	if args.Config.DryRun {
		fmt.Printf("Connection configuration:\n")
		fmt.Printf("  Host: %s\n", args.Config.Host)
		fmt.Printf("  Port: %d\n", args.Config.Port)
		fmt.Printf("  User: %s\n", args.Config.User)
		if args.Config.Database != "" {
			fmt.Printf("  Database: %s\n", args.Config.Database)
		} else {
			fmt.Printf("  Database: all databases\n")
		}
		fmt.Println()
	}

	// Create database connection
	ctx := context.Background()
	conn, err := database.Connect(ctx, args.Config.DatabaseURL())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Database connection failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Create role manager
	roleManager := role.NewManager(conn)

	// Execute role creation
	if args.Config.Database != "" {
		// Process specific database
		fmt.Printf("Processing specific database: %s\n", args.Config.Database)
		err = roleManager.CreateDatabaseRoles(args.Config.Database, args.Config.DryRun)
	} else {
		// Process all databases
		fmt.Println("Processing all accessible databases")
		err = roleManager.CreateAllDatabaseRoles(args.Config.DryRun)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Role creation failed: %v\n", err)
		os.Exit(1)
	}

	// Print completion message
	if args.Config.DryRun {
		fmt.Println("\nDry run completed successfully!")
		fmt.Println("No changes were made to the database.")
		fmt.Println("Remove --dry-run flag to execute the changes.")
	} else {
		fmt.Println("\nRole creation completed successfully!")
		fmt.Println("All roles have been created with appropriate permissions.")
	}

	// List created roles
	fmt.Println("\nCurrent pg-genrole managed roles:")
	if err := roleManager.ListRoles(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to list roles: %v\n", err)
	}
}
