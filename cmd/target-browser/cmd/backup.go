package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	targetBrowser "github.com/trilioData/tvk-plugins/tools/target-browser"
)

// nolint:lll // ignore long line lint errors
var backupCmd = &cobra.Command{
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

  # List of backups: filter by [tvkInstanceUID]
  kubectl tvk-target-browser get backup --tvk-instance-uid <uid> --target-name <name> --target-namespace <namespace>

  # List of backups: filter by [expirationStartDate] and [expirationEndDate]
  kubectl tvk-target-browser get backup --expiration-start-date <expiration-start-date> --expiration-end-date <expiration-end-date>--target-name <name> --target-namespace <namespace>

  # List of backups: filter by [creationStartDate] and [creationEndDate]
  kubectl tvk-target-browser get backup --creation-start-date <creation-start-date> --creation-end-date <creation-end-date>--target-name <name> --target-namespace <namespace>
`,

	RunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed(ExpirationStartDateFlag) && expirationEndDate == "" ||
			cmd.Flags().Changed(ExpirationEndDateFlag) && expirationEndDate == "" {
			return fmt.Errorf("[%s] flag value cannot be empty", ExpirationEndDateFlag)
		}
		var ts *time.Time
		var err error
		if expirationEndDate != "" {
			ts, err = parseTimestamp(expirationEndDate)
			if err != nil {
				return fmt.Errorf("[%s] flag invalid value. Usage - %s", ExpirationEndDateFlag, expirationEndDateUsage)
			}
			expirationEndDate = ts.Format(time.RFC3339)
		}

		if expirationStartDate != "" {
			ts, err = parseTimestamp(expirationStartDate)
			if err != nil {
				return fmt.Errorf("[%s] flag invalid value. Usage - %s", ExpirationStartDateFlag, expirationStartDateUsage)
			}
			expirationStartDate = ts.Format(time.RFC3339)
		}

		if cmd.Flags().Changed(ExpirationEndDateFlag) && expirationStartDate == "" {
			expirationStartDate = time.Now().Format(time.RFC3339)
		}
		return getBackupList(args)
	},
}

func init() {
	backupCmd.Flags().StringVar(&backupPlanUID, BackupPlanUIDFlag, backupPlanUIDDefault, backupPlanUIDUsage)
	backupCmd.Flags().StringVar(&backupStatus, BackupStatusFlag, backupStatusDefault, backupStatusUsage)
	backupCmd.Flags().StringVar(&backupUID, BackupUIDFlag, backupUIDDefault, backupUIDUsage)
	backupCmd.Flags().StringVar(&expirationStartDate, ExpirationStartDateFlag, "", expirationStartDateUsage)
	backupCmd.Flags().StringVar(&expirationEndDate, ExpirationEndDateFlag, "", expirationEndDateUsage)
	getCmd.AddCommand(backupCmd)
}

func getBackupList(args []string) error {

	if len(args) > 1 {
		args = removeDuplicates(args)
	}

	bpOptions := targetBrowser.BackupListOptions{
		BackupPlanUID:            backupPlanUID,
		BackupStatus:             backupStatus,
		CommonListOptions:        commonOptions,
		ExpirationStartTimestamp: expirationStartDate,
		ExpirationEndTimestamp:   expirationEndDate,
	}
	err := targetBrowserAuthConfig.GetBackups(&bpOptions, args)
	if err != nil {
		return err
	}
	return nil
}
