// Package config handles configuration management for pg-genrole
package config

import (
	"fmt"
	"strconv"

	"github.com/go-playground/validator/v10"
)

// Config holds the application configuration
type Config struct {
	// Database connection parameters
	Host     string `validate:"required" json:"host"`
	Port     int    `validate:"min=1,max=65535" json:"port"`
	User     string `validate:"required" json:"user"`
	Password string `validate:"required" json:"password"`
	Database string `json:"database"`

	// Application options
	DryRun bool `json:"dry_run"`
}

// Validate validates the configuration parameters using go-playground/validator
func (c *Config) Validate() error {
	validate := validator.New()
	return validate.Struct(c)
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
