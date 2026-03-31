// Package cmd implements the CLI commands for mm-ready.
package cmd

import (
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "mm-ready-go",
	Short: "Scan a PostgreSQL database for Spock 5 multi-master readiness",
	Long:  "mm-ready scans a PostgreSQL database and generates a compatibility report for pgEdge Spock 5 multi-master replication.",
	// Default to scan if no subcommand given but args are present
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
		os.Exit(1)
	},
}

func init() {
	rootCmd.Version = version

	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(auditCmd)
	rootCmd.AddCommand(monitorCmd)
	rootCmd.AddCommand(listChecksCmd)
	rootCmd.AddCommand(analyzeCmd)
}

// Execute runs the root command. Called from main().
func Execute() error {
	// Default to scan when first arg isn't a known command
	if len(os.Args) > 1 {
		firstArg := os.Args[1]
		knownCommands := map[string]bool{
			"scan": true, "audit": true, "monitor": true, "list-checks": true,
			"analyze": true, "help": true, "completion": true,
		}
		if !knownCommands[firstArg] && firstArg != "--version" && firstArg != "--help" && firstArg != "-h" && firstArg != "-v" {
			// Prepend "scan" to args
			os.Args = append([]string{os.Args[0], "scan"}, os.Args[1:]...)
		}
	}
	return rootCmd.Execute()
}

// Connection flags shared by scan, audit, and monitor commands.
type connFlags struct {
	// DSN is the full connection URI.
	DSN string
	// Host is the database server hostname.
	Host string
	// Port is the database server port.
	Port int
	// DBName is the database name.
	DBName string
	// User is the database user.
	User string
	// Password is the database password.
	Password string
	// SSLMode is the SSL connection mode.
	SSLMode string
	// SSLCert is the path to the SSL client certificate.
	SSLCert string
	// SSLKey is the path to the SSL client key.
	SSLKey string
	// SSLRootCert is the path to the SSL root CA certificate.
	SSLRootCert string
}

// Output flags shared by scan, audit, and monitor commands.
type outputFlags struct {
	// Format is the output format (json, markdown, html).
	Format string
	// Output is the output file path.
	Output string
}

func addConnFlags(cmd *cobra.Command, f *connFlags) {
	cmd.Flags().StringVar(&f.DSN, "dsn", "", "PostgreSQL connection URI (postgres://...)")
	cmd.Flags().StringVarP(&f.Host, "host", "H", envOrDefault("PGHOST", ""), "Database host")
	cmd.Flags().IntVarP(&f.Port, "port", "p", envIntOrDefault("PGPORT", 5432), "Database port")
	cmd.Flags().StringVarP(&f.DBName, "dbname", "d", envOrDefault("PGDATABASE", ""), "Database name")
	cmd.Flags().StringVarP(&f.User, "user", "U", envOrDefault("PGUSER", ""), "Database user")
	cmd.Flags().StringVarP(&f.Password, "password", "W", envOrDefault("PGPASSWORD", ""), "Database password")
	cmd.Flags().StringVar(&f.SSLMode, "sslmode", envOrDefault("PGSSLMODE", ""), "SSL mode (disable, require, verify-ca, verify-full)")
	cmd.Flags().StringVar(&f.SSLCert, "sslcert", envOrDefault("PGSSLCERT", ""), "Path to SSL client certificate")
	cmd.Flags().StringVar(&f.SSLKey, "sslkey", envOrDefault("PGSSLKEY", ""), "Path to SSL client key")
	cmd.Flags().StringVar(&f.SSLRootCert, "sslrootcert", envOrDefault("PGSSLROOTCERT", ""), "Path to SSL root certificate")
}

func addOutputFlags(cmd *cobra.Command, f *outputFlags) {
	cmd.Flags().StringVarP(&f.Format, "format", "f", "html", "Report format (json, markdown, html)")
	cmd.Flags().StringVarP(&f.Output, "output", "o", "", "Output file path (default: ./reports/<dbname>_<timestamp>.<ext>)")
}

var configPath string
var noConfig bool
var noTodo bool
var todoIncludeConsider bool

func addConfigFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&configPath, "config", "", "Path to config file (default: auto-discover)")
	cmd.Flags().BoolVar(&noConfig, "no-config", false, "Skip config file loading")
}

func addReportFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&noTodo, "no-todo", false, "Omit To Do list from report")
	cmd.Flags().BoolVar(&todoIncludeConsider, "todo-include-consider", false, "Include CONSIDER items in To Do list")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
