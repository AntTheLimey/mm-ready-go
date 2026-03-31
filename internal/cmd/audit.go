package cmd

import "github.com/spf13/cobra"

var auditConn connFlags
var auditOut outputFlags
var auditCategories string
var auditExclude string
var auditIncludeOnly string
var auditVerbose bool

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Post-Spock audit (target: database with Spock already installed)",
	RunE:  runAudit,
}

func init() {
	addConnFlags(auditCmd, &auditConn)
	addOutputFlags(auditCmd, &auditOut)
	addConfigFlags(auditCmd)
	addReportFlags(auditCmd)
	auditCmd.Flags().StringVar(&auditCategories, "categories", "", "Comma-separated list of check categories to run")
	auditCmd.Flags().StringVar(&auditExclude, "exclude", "", "Comma-separated list of check names to skip")
	auditCmd.Flags().StringVar(&auditIncludeOnly, "include-only", "", "Comma-separated list of check names to run (whitelist)")
	auditCmd.Flags().BoolVarP(&auditVerbose, "verbose", "v", false, "Print progress")
}

func runAudit(cmd *cobra.Command, args []string) error {
	return runMode(auditConn, auditOut, auditCategories, auditExclude, auditIncludeOnly, auditVerbose, "audit")
}
