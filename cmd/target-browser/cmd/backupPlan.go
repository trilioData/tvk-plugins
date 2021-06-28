package cmd

import (
	"github.com/spf13/cobra"

	targetBrowser "github.com/trilioData/tvk-plugins/tools/target-browser"
)

func init() {
	getCmd.AddCommand(backupPlanCmd())
}

func backupPlanCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     BackupPlanCmdName,
		Aliases: []string{backupPlanCmdPluralName, backupPlanCmdAlias, backupPlanCmdAliasPlural},
		Short:   "API to perform Read operations on BackupPlans",
		Long: `API to perform Read operations on BackupPlans. Get a list of BackupPlans from target with using options.
  Order backupPlan in ascending or descending order,
  Filter backupPlan using flag tvk-instance-uid.`,
		Example: `  # List of backupPlans	
  kubectl tvk-target-browser get backupPlan

  # List of backupPlans order by backupPlan name
  kubectl tvk-target-browser get backupPlan --order-by name

  # List of backupPlans order by backupPlan Timestamp in ascending order
  kubectl tvk-target-browser get backupPlan --order-by backupTimestamp 

  # List of backupPlans order by backupPlan Timestamp in descending order
  kubectl tvk-target-browser get backupPlan --order-by -backupTimestamp
  
  # List of backupPlans filter by tvkInstanceUID
  kubectl tvk-target-browser get backupPlan --tvk-instance-uid <uid>
`,
		RunE: getBackupPlanList,
	}

	cmd.Flags().StringVar(&tvkInstanceUID, TvkInstanceUIDFlag, tvkInstanceUIDDefault, tvkInstanceUIDUsage)
	return cmd
}

func getBackupPlanList(*cobra.Command, []string) error {
	bpOptions := targetBrowser.BackupPlanListOptions{
		CommonListOptions: commonOptions,
		TvkInstanceUID:    tvkInstanceUID,
	}
	err := targetBrowserAuthConfig.GetBackupPlans(&bpOptions)
	if err != nil {
		return err
	}
	return nil
}
