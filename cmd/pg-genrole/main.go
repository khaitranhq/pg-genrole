package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/khaitranhq/pg-genrole/internal/service"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var (
		host      = flag.String("host", "", "PostgreSQL host")
		port      = flag.Int("port", 0, "PostgreSQL port")
		user      = flag.String("user", "", "PostgreSQL user")
		password  = flag.String("password", "", "PostgreSQL password")
		databases = flag.String("databases", "", "comma-separated databases to process; all databases if empty")
		debug     = flag.Bool("debug", false, "enable debug logging")
	)
	flag.Parse()

	if *host == "" || *user == "" || *password == "" || *port == 0 {
		flag.Usage()
		return fmt.Errorf("host, port, user and password are required")
	}

	var dbList []string
	if *databases != "" {
		dbList = strings.Split(*databases, ",")
	}

	manager := service.NewDatabaseManager(service.Config{
		Host:     *host,
		Port:     *port,
		User:     *user,
		Password: *password,
	}, *debug)

	if err := manager.RefreshRolesPermissions(context.Background(), dbList); err != nil {
		return fmt.Errorf("refresh roles permissions: %w", err)
	}

	return nil
}
