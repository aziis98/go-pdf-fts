package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Config holds global application configuration
type Config struct {
	DBPath  string
	Verbose bool
}

// New creates a new configuration with defaults
func New() *Config {
	return &Config{}
}

// FindExistingDBPath searches for an existing database file up the directory tree
func (c *Config) FindExistingDBPath() error {
	dbName := "fts.db"

	// Start from current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Search up the directory tree until we reach the root
	for {
		dbPath := filepath.Join(currentDir, dbName)
		log.Printf("Searching for database at: %s\n", dbPath) // Add logging here
		if _, err := os.Stat(dbPath); err == nil {
			// Found the database file
			c.DBPath = dbPath
			log.Printf("Found database at: %s\n", dbPath) // Add logging here
			return nil
		}

		// Move to parent directory
		parentDir := filepath.Dir(currentDir)

		// Check if we've reached the root (parent is the same as current)
		if parentDir == currentDir {
			break
		}

		currentDir = parentDir
	}

	return fmt.Errorf("no existing database found in directory tree")
}

// CreateDBPath creates a new database path in the current directory
func (c *Config) CreateDBPath() error {
	dbName := "fts.db"

	absPath, err := filepath.Abs(dbName)
	if err != nil {
		return fmt.Errorf("failed to create database path: %w", err)
	}
	c.DBPath = absPath
	return nil
}

// FindOrCreateDBPath finds an existing database or creates a new one
func (c *Config) FindOrCreateDBPath() error {
	// First try to find existing database
	if err := c.FindExistingDBPath(); err == nil {
		return nil
	}

	// If not found, create a new path
	return c.CreateDBPath()
}
