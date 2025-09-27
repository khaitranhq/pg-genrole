// Package config handles configuration management for pg-genrole
package config

import (
	"fmt"
	"strconv"
)

// Config holds the application configuration
type Config struct {
	// Database connection parameters
	Host     string
	Port     int
	User     string
	Password string
	Database string

	// Application options
	DryRun bool
}

// Validate validates the configuration parameters
func (c *Config) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if c.User == "" {
		return fmt.Errorf("database user is required")
	}
	if c.Password == "" {
		return fmt.Errorf("database password is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port out of range: %d (must be 1-65535)", c.Port)
	}
	return nil
}

// DatabaseURL returns a PostgreSQL connection URL
func (c *Config) DatabaseURL() string {
	dbName := c.Database
	if dbName == "" {
		dbName = "postgres"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=prefer",
		c.User, c.Password, c.Host, c.Port, dbName)
}

// String returns a safe string representation of the config (without password)
func (c *Config) String() string {
	return fmt.Sprintf("Config{Host: %s, Port: %d, User: %s, Database: %s, DryRun: %t}",
		c.Host, c.Port, c.User, c.Database, c.DryRun)
}

// SetDefaults sets default values for configuration
func (c *Config) SetDefaults() {
	if c.Port == 0 {
		c.Port = 5432
	}
}

// ParsePort parses a port string into an integer
func ParsePort(portStr string) (int, error) {
	if portStr == "" {
		return 5432, nil
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port number: %s", portStr)
	}

	if port <= 0 || port > 65535 {
		return 0, fmt.Errorf("port must be between 1 and 65535")
	}

	return port, nil
}
