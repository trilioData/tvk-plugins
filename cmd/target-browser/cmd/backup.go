package cmd

import (
	"fmt"

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


  # List of backups: filter by [expirationStartTimestamp] and [expirationEndTimestamp]
  kubectl tvk-target-browser get backup --expiration-start-timestamp <expiration-start-timestamp> --expiration-end-timestamp <expiration-end-timestamp>--target-name <name> --target-namespace <namespace>

  # List of backups: filter by [tvkInstanceUID]
  kubectl tvk-target-browser get backupPlan --tvk-instance-uid <uid> --target-name <name> --target-namespace <namespace>


  # List of backups: filter by [creationStartTimestamp] and [creationEndTimestamp]
  kubectl tvk-target-browser get backup --creation-start-timestamp <creation-start-timestamp> --creation-end-timestamp <creation-end-timestamp>--target-name <name> --target-namespace <namespace>
`,

	RunE: func(cmd *cobra.Command, args []string) error {
		err := validateBackupCmdInput()
		if err != nil {
			return err
		}
		return getBackupList(args)
	},
}

func init() {
	backupCmd.Flags().StringVar(&backupPlanUID, BackupPlanUIDFlag, backupPlanUIDDefault, backupPlanUIDUsage)
	backupCmd.Flags().StringVar(&backupStatus, BackupStatusFlag, backupStatusDefault, backupStatusUsage)
	backupCmd.Flags().StringVar(&backupUID, BackupUIDFlag, backupUIDDefault, backupUIDUsage)
	backupCmd.Flags().StringVar(&expirationStartTimestamp, ExpirationStartTimestampFlag, "", expirationStartTimestampUsage)
	backupCmd.Flags().StringVar(&expirationEndTimestamp, ExpirationEndTimestampFlag, "", expirationEndTimestampUsage)
	getCmd.AddCommand(backupCmd)

}

func validateBackupCmdInput() error {
	if expirationStartTimestamp != "" && expirationEndTimestamp != "" {
		expirationStartTimestamp = parseTimestamp(expirationStartTimestamp, StartTime)
		expirationEndTimestamp = parseTimestamp(expirationEndTimestamp, EndTime)
	} else if expirationEndTimestamp != "" {
		expirationEndTimestamp = parseTimestamp(expirationEndTimestamp, EndTime)
		expirationStartTimestamp = extractDate(expirationEndTimestamp) + "T" + StartTime + "Z"
	} else if expirationStartTimestamp != "" {
		expirationStartTimestamp = parseTimestamp(expirationStartTimestamp, StartTime)
		expirationEndTimestamp = extractDate(expirationStartTimestamp) + "T" + EndTime + "Z"
	}

	if expirationStartTimestamp != "" && !validateRFC3339Timestamps(expirationStartTimestamp) {
		return fmt.Errorf("[%s] flag invalid value. Usage - %s", ExpirationStartTimestampFlag, expirationStartTimestampUsage)
	}
	if expirationEndTimestamp != "" && !validateRFC3339Timestamps(expirationEndTimestamp) {
		return fmt.Errorf("[%s] flag invalid value. Usage - %s", ExpirationEndTimestampFlag, expirationEndTimestampUsage)
	}
	return nil
}

func getBackupList(args []string) error {

	if len(args) > 1 {
		args = removeDuplicates(args)
	}

	bpOptions := targetBrowser.BackupListOptions{
		BackupPlanUID:            backupPlanUID,
		BackupStatus:             backupStatus,
		CommonListOptions:        commonOptions,
		ExpirationStartTimestamp: expirationStartTimestamp,
		ExpirationEndTimestamp:   expirationEndTimestamp,
	}
	err := targetBrowserAuthConfig.GetBackups(&bpOptions, args)
	if err != nil {
		return err
	}
	return nil
}
