// Package cmd implements the CLI commands for mm-ready.
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "mm-ready",
	Short: "Scan a PostgreSQL database for Spock 5 multi-master readiness",
	Long:  "mm-ready scans a PostgreSQL database and generates a compatibility report for pgEdge Spock 5 multi-master replication.",
	// Default to scan if no subcommand given but args are present
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
		os.Exit(1)
	},
}

func init() {
	rootCmd.Version = version

	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(auditCmd)
	rootCmd.AddCommand(monitorCmd)
	rootCmd.AddCommand(listChecksCmd)
}

// Execute runs the root command. Called from main().
func Execute() error {
	// Default to scan when first arg isn't a known command
	if len(os.Args) > 1 {
		firstArg := os.Args[1]
		knownCommands := map[string]bool{
			"scan": true, "audit": true, "monitor": true, "list-checks": true,
			"help": true, "completion": true,
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
	DSN      string
	Host     string
	Port     int
	DBName   string
	User     string
	Password string
}

// Output flags shared by scan, audit, and monitor commands.
type outputFlags struct {
	Format string
	Output string
}

func addConnFlags(cmd *cobra.Command, f *connFlags) {
	cmd.Flags().StringVar(&f.DSN, "dsn", "", "PostgreSQL connection URI (postgres://...)")
	cmd.Flags().StringVarP(&f.Host, "host", "H", "", "Database host")
	cmd.Flags().IntVarP(&f.Port, "port", "p", 5432, "Database port")
	cmd.Flags().StringVarP(&f.DBName, "dbname", "d", "", "Database name")
	cmd.Flags().StringVarP(&f.User, "user", "U", "", "Database user")
	cmd.Flags().StringVarP(&f.Password, "password", "W", "", "Database password")
}

func addOutputFlags(cmd *cobra.Command, f *outputFlags) {
	cmd.Flags().StringVarP(&f.Format, "format", "f", "html", "Report format (json, markdown, html)")
	cmd.Flags().StringVarP(&f.Output, "output", "o", "", "Output file path (default: ./reports/<dbname>_<timestamp>.<ext>)")
}
