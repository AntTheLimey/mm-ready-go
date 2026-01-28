package cmd

import "github.com/spf13/cobra"

var auditConn connFlags
var auditOut outputFlags
var auditCategories string
var auditVerbose bool

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Post-Spock audit (target: database with Spock already installed)",
	RunE:  runAudit,
}

func init() {
	addConnFlags(auditCmd, &auditConn)
	addOutputFlags(auditCmd, &auditOut)
	auditCmd.Flags().StringVar(&auditCategories, "categories", "", "Comma-separated list of check categories to run")
	auditCmd.Flags().BoolVarP(&auditVerbose, "verbose", "v", false, "Print progress")
}

func runAudit(cmd *cobra.Command, args []string) error {
	return runMode(auditConn, auditOut, auditCategories, auditVerbose, "audit")
}
