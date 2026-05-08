package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"gopkg.in/yaml.v3"
	
	"reporting_app/internal/core"
	"reporting_app/internal/logging"
)

// ConnectionConfig defines a single database connection
type ConnectionConfig struct {
	Driver         string `yaml:"driver"`
	DSN            string `yaml:"dsn"`
	MaxConnections int    `yaml:"max_connections"`
	ReadOnly       bool   `yaml:"read_only"`
	Timeout        string `yaml:"timeout"`
	RowLimit       int    `yaml:"row_limit"`
}

// Config holds all database connections
type Config struct {
	Connections map[string]ConnectionConfig `yaml:"connections"`
}

// Manager holds connection pools for all configured databases
type Manager struct {
	config  *Config
	clients map[string]*sql.DB
	logger  *logging.QueryLogger
}

// NewManager creates a new database manager by loading config from path
func NewManager(configPath string) (*Manager, error) {
	return NewManagerWithLogger(configPath, nil)
}

// NewManagerWithLogger creates a new database manager with query logger
func NewManagerWithLogger(configPath string, logger *logging.QueryLogger) (*Manager, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read database config %s: %w", configPath, err)
	}
	
	// Expand environment variables
	expanded := os.ExpandEnv(string(data))
	
	var config Config
	if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
		return nil, fmt.Errorf("failed to parse database config YAML: %w", err)
	}
	
	if len(config.Connections) == 0 {
		return nil, fmt.Errorf("no database connections defined")
	}
	
	m := &Manager{
		config:  &config,
		clients: make(map[string]*sql.DB),
		logger:  logger,
	}
	
	// Initialize each connection
	for name, conn := range config.Connections {
		if conn.Driver != "mysql" {
			return nil, fmt.Errorf("unsupported driver %q for connection %q", conn.Driver, name)
		}
		
		db, err := sql.Open(conn.Driver, conn.DSN)
		if err != nil {
			return nil, fmt.Errorf("failed to open database %q: %w", name, err)
		}
		
		// Set connection pool settings
		if conn.MaxConnections > 0 {
			db.SetMaxOpenConns(conn.MaxConnections)
			db.SetMaxIdleConns(conn.MaxConnections / 2)
		}
		
		// Parse timeout duration
		timeout := 30 * time.Second
		if conn.Timeout != "" {
			d, err := time.ParseDuration(conn.Timeout)
			if err != nil {
				log.Printf("invalid timeout %q for %q, using default 30s: %v", conn.Timeout, name, err)
			} else {
				timeout = d
			}
		}
		db.SetConnMaxLifetime(timeout)
		
		// Ping to verify connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			log.Printf("warning: database %q ping failed: %v", name, err)
			// Continue anyway; maybe database will be available later
		}
		
		m.clients[name] = db
		log.Printf("Database connection %q ready", name)
	}
	
	return m, nil
}

// GetClient returns a database client for the given connection name
func (m *Manager) GetClient(name string) (*sql.DB, error) {
	db, ok := m.clients[name]
	if !ok {
		return nil, fmt.Errorf("database connection %q not found", name)
	}
	return db, nil
}

// CloseAll closes all database connections
func (m *Manager) CloseAll() error {
	var errs []string
	for name, db := range m.clients {
		if err := db.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing databases: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ConnectionConfig returns the config for a specific connection
func (m *Manager) ConnectionConfig(name string) (*ConnectionConfig, error) {
	cfg, ok := m.config.Connections[name]
	if !ok {
		return nil, fmt.Errorf("database connection %q not found", name)
	}
	return &cfg, nil
}

// ExecuteDatasourceManager is a convenience function that uses the manager's logger
func (m *Manager) ExecuteDatasource(db *sql.DB, ds core.Datasource, params map[string]string, defaultRowLimit int, timeout time.Duration, reportID, datasource, dbName string) (*core.QueryResult, error) {
	return ExecuteDatasourceWithLogger(db, ds, params, defaultRowLimit, timeout, reportID, datasource, dbName, m.logger)
}