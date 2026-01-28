package cmd

import (
	"context"

	"github.com/AntTheLimey/mm-ready/internal/connection"
	"github.com/AntTheLimey/mm-ready/internal/monitor"
	"github.com/AntTheLimey/mm-ready/internal/reporter"
	"github.com/spf13/cobra"
)

var monitorConn connFlags
var monitorOut outputFlags
var monitorDuration int
var monitorLogFile string
var monitorVerbose bool

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Observe SQL activity over a time window then report",
	RunE:  runMonitor,
}

func init() {
	addConnFlags(monitorCmd, &monitorConn)
	addOutputFlags(monitorCmd, &monitorOut)
	monitorCmd.Flags().IntVar(&monitorDuration, "duration", 3600, "Observation duration in seconds")
	monitorCmd.Flags().StringVar(&monitorLogFile, "log-file", "", "Path to PostgreSQL log file")
	monitorCmd.Flags().BoolVarP(&monitorVerbose, "verbose", "v", false, "Print progress")
}

func runMonitor(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	conn, err := connection.Connect(ctx, connection.Config{
		Host:     monitorConn.Host,
		Port:     monitorConn.Port,
		DBName:   monitorConn.DBName,
		User:     monitorConn.User,
		Password: monitorConn.Password,
		DSN:      monitorConn.DSN,
	})
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	report, err := monitor.RunMonitor(ctx, conn, monitor.Options{
		Host:     monitorConn.Host,
		Port:     monitorConn.Port,
		DBName:   monitorConn.DBName,
		Duration: monitorDuration,
		LogFile:  monitorLogFile,
		Verbose:  monitorVerbose,
	})
	if err != nil {
		return err
	}

	output, err := reporter.Render(report, monitorOut.Format)
	if err != nil {
		return err
	}

	return writeOutput(output, monitorOut, report.Database)
}
