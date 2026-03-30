// Package connection provides PostgreSQL database connection management.
package connection

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

// Config holds the parameters needed to connect to a PostgreSQL database.
type Config struct {
	Host        string
	Port        int
	DBName      string
	User        string
	Password    string
	DSN         string
	SSLMode     string
	SSLCert     string
	SSLKey      string
	SSLRootCert string
}

// Connect creates a database connection from a Config.
func Connect(ctx context.Context, cfg Config) (*pgx.Conn, error) {
	var connStr string

	if cfg.DSN != "" {
		connStr = cfg.DSN
	} else {
		connStr = buildConnString(cfg)
	}

	connConfig, err := pgx.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parse connection config: %w", err)
	}
	connConfig.RuntimeParams["default_transaction_read_only"] = "on"

	if cfg.SSLMode != "" && cfg.SSLMode != "disable" {
		tlsConfig, tlsErr := buildTLSConfig(cfg)
		if tlsErr != nil {
			return nil, fmt.Errorf("configure TLS: %w", tlsErr)
		}
		connConfig.TLSConfig = tlsConfig
	}

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
	if cfg.SSLMode != "" {
		parts += fmt.Sprintf("sslmode=%s ", cfg.SSLMode)
	}
	return parts
}

func buildTLSConfig(cfg Config) (*tls.Config, error) {
	tlsCfg := &tls.Config{}

	switch cfg.SSLMode {
	case "require":
		tlsCfg.InsecureSkipVerify = true
	case "verify-ca", "verify-full":
		tlsCfg.InsecureSkipVerify = false
		if cfg.SSLMode == "verify-full" {
			tlsCfg.ServerName = cfg.Host
		}
	default:
		tlsCfg.InsecureSkipVerify = true
	}

	if cfg.SSLRootCert != "" {
		caCert, err := os.ReadFile(cfg.SSLRootCert)
		if err != nil {
			return nil, fmt.Errorf("read SSL root cert: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse SSL root cert")
		}
		tlsCfg.RootCAs = pool
	}

	if cfg.SSLCert != "" && cfg.SSLKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.SSLCert, cfg.SSLKey)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}
