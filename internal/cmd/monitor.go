package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pgEdge/mm-ready-go/internal/config"
	"github.com/pgEdge/mm-ready-go/internal/connection"
	"github.com/pgEdge/mm-ready-go/internal/monitor"
	"github.com/pgEdge/mm-ready-go/internal/reporter"
	"github.com/spf13/cobra"
)

var monitorConn connFlags
var monitorOut outputFlags
var monitorDuration int
var monitorLogFile string
var monitorExclude string
var monitorIncludeOnly string
var monitorVerbose bool

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Observe SQL activity over a time window then report",
	RunE:  runMonitor,
}

func init() {
	addConnFlags(monitorCmd, &monitorConn)
	addOutputFlags(monitorCmd, &monitorOut)
	addConfigFlags(monitorCmd)
	monitorCmd.Flags().IntVar(&monitorDuration, "duration", 3600, "Observation duration in seconds")
	monitorCmd.Flags().StringVar(&monitorLogFile, "log-file", "", "Path to PostgreSQL log file")
	monitorCmd.Flags().StringVar(&monitorExclude, "exclude", "", "Comma-separated list of check names to skip")
	monitorCmd.Flags().StringVar(&monitorIncludeOnly, "include-only", "", "Comma-separated list of check names to run (whitelist)")
	monitorCmd.Flags().BoolVarP(&monitorVerbose, "verbose", "v", false, "Print progress")
}

func runMonitor(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	conn, err := connection.Connect(ctx, connection.Config{
		Host:        monitorConn.Host,
		Port:        monitorConn.Port,
		DBName:      monitorConn.DBName,
		User:        monitorConn.User,
		Password:    monitorConn.Password,
		DSN:         monitorConn.DSN,
		SSLMode:     monitorConn.SSLMode,
		SSLCert:     monitorConn.SSLCert,
		SSLKey:      monitorConn.SSLKey,
		SSLRootCert: monitorConn.SSLRootCert,
	})
	if err != nil {
		return formatConnError(err, monitorConn)
	}
	defer conn.Close(ctx)

	// Load config
	var cfg config.Config
	if !noConfig {
		if configPath != "" {
			cfg, err = config.LoadFile(configPath)
			if err != nil {
				return err
			}
		} else {
			path := config.DiscoverConfigFile()
			if path != "" {
				cfg, err = config.LoadFile(path)
				if err != nil {
					return err
				}
				if monitorVerbose {
					fmt.Fprintf(os.Stderr, "Using config file: %s\n", path)
				}
			} else {
				cfg = config.Default()
			}
		}
	} else {
		cfg = config.Default()
	}

	checkCfg, _ := config.MergeCLI(cfg, "monitor", splitComma(monitorExclude), splitComma(monitorIncludeOnly), false, false)

	report, err := monitor.RunMonitor(ctx, conn, monitor.Options{
		Host:        monitorConn.Host,
		Port:        monitorConn.Port,
		DBName:      monitorConn.DBName,
		Duration:    monitorDuration,
		LogFile:     monitorLogFile,
		Verbose:     monitorVerbose,
		Exclude:     checkCfg.Exclude,
		IncludeOnly: checkCfg.IncludeOnly,
	})
	if err != nil {
		return err
	}

	output, err := reporter.Render(report, monitorOut.Format, reporter.DefaultReportOptions())
	if err != nil {
		return err
	}

	return writeOutput(output, monitorOut, report.Database)
}
