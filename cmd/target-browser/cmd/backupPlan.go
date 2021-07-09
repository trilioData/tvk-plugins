package cmd

import (
	"github.com/spf13/cobra"

	targetBrowser "github.com/trilioData/tvk-plugins/tools/target-browser"
)

func init() {
	getCmd.AddCommand(backupPlanCmd())
}

// nolint:lll // ignore long line lint errors
// backupPlanCmd represents the backupPlan command
func backupPlanCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     BackupPlanCmdName,
		Aliases: []string{backupPlanCmdPluralName, backupPlanCmdAlias, backupPlanCmdAliasPlural},
		Short:   "Get specific BackupPlan or list of BackupPlans",
		Long: `Performs GET operation on target-browser's '/backupplan' API and gets specific BackupPlan or list of BackupPlans from mounted target location
using available flags and options.`,
		Example: `  # List of backupPlans	
  kubectl tvk-target-browser get backupPlan --target-name <name> --target-namespace <namespace>

  # Get specific backupPlan
  kubectl tvk-target-browser get backupPlan <backup-plan-uid> --target-name <name> --target-namespace <namespace>

  # List of backupPlans: order by [name]
  kubectl tvk-target-browser get backupPlan --order-by name --target-name <name> --target-namespace <namespace>

  # List of backupPlans: order by [backupPlanTimestamp] in [ascending] order
  kubectl tvk-target-browser get backupPlan --order-by backupPlanTimestamp --target-name <name> --target-namespace <namespace>

  # List of backupPlans: order by [backupPlanTimestamp] in [descending] order
  kubectl tvk-target-browser get backupPlan --order-by -backupPlanTimestamp --target-name <name> --target-namespace <namespace>
  
  # List of backupPlans: filter by [tvkInstanceUID]
  kubectl tvk-target-browser get backupPlan --tvk-instance-uid <uid> --target-name <name> --target-namespace <namespace>
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return getBackupPlanList(args)
		},
	}
	cmd.Flags().StringVar(&tvkInstanceUID, TvkInstanceUIDFlag, tvkInstanceUIDDefault, tvkInstanceUIDUsage)
	return cmd
}

func getBackupPlanList(args []string) error {
	bpOptions := targetBrowser.BackupPlanListOptions{
		CommonListOptions: commonOptions,
		TvkInstanceUID:    tvkInstanceUID,
	}
	err := targetBrowserAuthConfig.GetBackupPlans(&bpOptions, args)
	if err != nil {
		return err
	}
	return nil
}
