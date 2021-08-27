package cmd

import (
	"github.com/spf13/cobra"

	targetBrowser "github.com/trilioData/tvk-plugins/tools/target-browser"
)

func init() {
	backupCmd.AddCommand(trilioResourcesCmd())
}

// nolint:lll // ignore long line lint errors
// trilioResourcesCmd represents the trilio-resources command (sub-command for backup)
func trilioResourcesCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   TrilioResourcesCmdName,
		Short: "Get trilio resources for specific backup",
		Long:  `Performs GET operation on target-browser's '/backup/{backupUID}/trilio-resources' API and gets trilio resources for backup`,
		Example: `  # Get trilio resources for specific backup
  kubectl tvk-target-browser get backup trilio-resources <backup-uid> --backup-plan-uid <uid> --kinds ClusterBackup, ClusterBackupPlan," +
		" Backup, BackupPlan, Target, Secret, Policy, Hook`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return getTrilioResources(args)
		},
	}

	cmd.Flags().StringSliceVar(&kinds, kindsFlag, []string{}, kindsUsage)

	cmd.Flags().StringVar(&backupPlanUID, BackupPlanUIDFlag, backupPlanUIDDefault, backupPlanUIDUsage)

	return cmd
}

func getTrilioResources(args []string) error {
	trilioResourcesOptions := targetBrowser.TrilioResourcesListOptions{
		BackupPlanUID:     backupPlanUID,
		BackupUID:         backupUID,
		Kinds:             kinds,
		CommonListOptions: commonOptions,
	}

	err := targetBrowserAuthConfig.GetTrilioResources(&trilioResourcesOptions, args)
	if err != nil {
		return err
	}
	return nil

}
