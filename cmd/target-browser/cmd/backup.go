package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	targetBrowser "github.com/trilioData/tvk-plugins/tools/target-browser"
)

func init() {
	getCmd.AddCommand(backupCmd())
}

// backupCmd represents the backup command
func backupCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     BackupCmdName,
		Aliases: []string{backupCmdPluralName},
		Short:   "Get specific Backup or list of Backups",
		Long: `Performs GET operation on target-browser's '/backup' API and gets specific Backup or list of Backups from mounted
target location for specific backupPlan using available flags and options.`,
		Example: `  # List of backups
  kubectl tvk-target-browser get backup --backup-plan-uid <uid>

  # Get specific backup (NOT SUPPORTED)
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --backup-uid <uid>

  # List of backups: order by [name]
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --order-by name

  # List of backups: order by [backupTimestamp] in [ascending] order
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --order-by backupTimestamp 

  # List of backups: order by [backupTimestamp] in [descending] order
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --order-by -backupTimestamp

  # List of backups: filter by [backup-status]
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --backup-status Available

  # List of backups: filter by [expiry-date] (NOT SUPPORTED)
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --expiry-date <date>

  # List of backups: filter by [creation-date] (NOT SUPPORTED)
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --creation-date <date>
`,
		RunE: getBackupList,
	}

	cmd.Flags().StringVar(&backupPlanUID, BackupPlanUIDFlag, backupPlanUIDDefault, backupPlanUIDUsage)
	err := cmd.MarkFlagRequired(BackupPlanUIDFlag)
	if err != nil {
		log.Fatalf("Invalid option or missing required flag %s and Error is %s", BackupPlanUIDFlag, err.Error())
	}
	cmd.Flags().StringVar(&backupStatus, BackupStatusFlag, backupStatusDefault, backupStatusUsage)
	cmd.Flags().StringVar(&backupUID, BackupUIDFlag, backupUIDDefault, backupUIDUsage)
	cmd.Flags().StringVar(&creationDate, creationDateFlag, creationDateDefault, creationDateUsage)
	cmd.Flags().StringVar(&expiryDate, expiryDateFlag, expiryDateDefault, expiryDateUsage)

	return cmd
}

func getBackupList(*cobra.Command, []string) error {
	bpOptions := targetBrowser.BackupListOptions{
		BackupPlanUID:     backupPlanUID,
		BackupStatus:      backupStatus,
		CommonListOptions: commonOptions,
	}
	err := targetBrowserAuthConfig.GetBackups(&bpOptions)
	if err != nil {
		return err
	}
	return nil
}
