package cmd

import (
	"github.com/spf13/cobra"

	targetBrowser "github.com/trilioData/tvk-plugins/tools/target-browser"
)

func init() {
	getCmd.AddCommand(backupCmd())
}

// nolint:lll // ignore long line lint errors
// backupCmd represents the backup command
func backupCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     BackupCmdName,
		Aliases: []string{backupCmdPluralName},
		Short:   "Get specific Backup or list of Backups",
		Long: `Performs GET operation on target-browser's '/backup' API and gets specific Backup or list of Backups from mounted target location
for specific backupPlan using available flags and options.`,
		Example: `  # List of backups for specific backupPlan
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --target-name <name> --target-namespace <namespace>

  # List of backups
  kubectl tvk-target-browser get backup --target-name <name> --target-namespace <namespace>
  
  # Get specific backup
  kubectl tvk-target-browser get backup <backup-uid> --target-name <name> --target-namespace <namespace>

  # List of backups: order by [name]
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --order-by name --target-name <name> --target-namespace <namespace>

  # List of backups: order by [backupTimestamp] in [ascending] order
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --order-by backupTimestamp --target-name <name> --target-namespace <namespace>

  # List of backups: order by [backupTimestamp] in [descending] order
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --order-by -backupTimestamp --target-name <name> --target-namespace <namespace>

  # List of backups: filter by [backup-status]
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --backup-status Available --target-name <name> --target-namespace <namespace>

  # List of backups in Single Namespace: filter by [operationScope]
  kubectl tvk-target-browser get backup --operation-scope SingleNamespace --target-name <name> --target-namespace <namespace>

  # List of backups in Multi Namespace/Cluster Scope: filter by [operationScope]
  kubectl tvk-target-browser get backup --operation-scope MultiNamespace --target-name <name> --target-namespace <namespace>

  # List of backups: filter by [expiry-date] (NOT SUPPORTED)
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --expiry-date <date> --target-name <name> --target-namespace <namespace>

  # List of backups: filter by [creation-date] (NOT SUPPORTED)
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --creation-date <date> --target-name <name> --target-namespace <namespace>
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return getBackupList(args)
		},
	}

	cmd.Flags().StringVar(&backupPlanUID, BackupPlanUIDFlag, backupPlanUIDDefault, backupPlanUIDUsage)
	cmd.Flags().StringVar(&backupStatus, BackupStatusFlag, backupStatusDefault, backupStatusUsage)
	cmd.Flags().StringVar(&backupUID, BackupUIDFlag, backupUIDDefault, backupUIDUsage)
	cmd.Flags().StringVar(&creationDate, creationDateFlag, creationDateDefault, creationDateUsage)
	cmd.Flags().StringVar(&expiryDate, expiryDateFlag, expiryDateDefault, expiryDateUsage)

	return cmd
}

func getBackupList(args []string) error {
	bpOptions := targetBrowser.BackupListOptions{
		BackupPlanUID:     backupPlanUID,
		BackupStatus:      backupStatus,
		CommonListOptions: commonOptions,
	}
	err := targetBrowserAuthConfig.GetBackups(&bpOptions, args)
	if err != nil {
		return err
	}
	return nil
}
