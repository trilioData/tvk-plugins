package cmd

import (
	log "github.com/sirupsen/logrus"
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
  kubectl tvk-target-browser get backup trilio-resources --backup-uid <uid> --backup-plan-uid <uid> --operation-scope <scope> --kinds <kinds>`,
		RunE: getTrilioResources,
	}

	cmd.Flags().StringVar(&operationScope, OperationScopeFlag, "", operationScopeUsage)

	cmd.Flags().StringSliceVar(&kinds, kindsFlag, []string{}, kindsUsage)

	cmd.Flags().StringVar(&backupPlanUID, BackupPlanUIDFlag, backupPlanUIDDefault, backupPlanUIDUsage)

	cmd.Flags().StringVar(&backupUID, BackupUIDFlag, backupUIDDefault, backupUIDUsage)
	err := cmd.MarkFlagRequired(BackupUIDFlag)
	if err != nil {
		log.Fatalf("Invalid option or missing required flag %s - %s", BackupUIDFlag, err.Error())
	}

	return cmd
}

func getTrilioResources(*cobra.Command, []string) error {
	trilioResourcesOptions := targetBrowser.TrilioResourcesListOptions{
		BackupPlanUID:     backupPlanUID,
		BackupUID:         backupUID,
		Kinds:             kinds,
		CommonListOptions: commonOptions,
	}

	err := targetBrowserAuthConfig.GetTrilioResources(&trilioResourcesOptions)
	if err != nil {
		return err
	}
	return nil

}
