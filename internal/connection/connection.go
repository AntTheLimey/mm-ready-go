// Package connection provides PostgreSQL database connection management.
package connection

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Config holds the parameters needed to connect to a PostgreSQL database.
type Config struct {
	Host     string
	Port     int
	DBName   string
	User     string
	Password string
	DSN      string
}

// Connect creates a database connection from a Config.
// If DSN is provided, it is used directly. Otherwise individual parameters are
// used, falling back to standard PG* environment variables (handled by pgx).
func Connect(ctx context.Context, cfg Config) (*pgx.Conn, error) {
	var connStr string

	if cfg.DSN != "" {
		connStr = cfg.DSN
	} else {
		connStr = buildConnString(cfg)
	}

	// Parse the connection string and add read-only runtime param
	connConfig, err := pgx.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parse connection config: %w", err)
	}
	connConfig.RuntimeParams["default_transaction_read_only"] = "on"

	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	return conn, nil
}

// GetPGVersion returns the PostgreSQL server version string.
func GetPGVersion(ctx context.Context, conn *pgx.Conn) (string, error) {
	var version string
	err := conn.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("get pg version: %w", err)
	}
	return version, nil
}

func buildConnString(cfg Config) string {
	parts := ""
	if cfg.Host != "" {
		parts += fmt.Sprintf("host=%s ", cfg.Host)
	}
	if cfg.Port != 0 {
		parts += fmt.Sprintf("port=%d ", cfg.Port)
	}
	if cfg.DBName != "" {
		parts += fmt.Sprintf("dbname=%s ", cfg.DBName)
	}
	if cfg.User != "" {
		parts += fmt.Sprintf("user=%s ", cfg.User)
	}
	if cfg.Password != "" {
		parts += fmt.Sprintf("password=%s ", cfg.Password)
	}
	return parts
}
