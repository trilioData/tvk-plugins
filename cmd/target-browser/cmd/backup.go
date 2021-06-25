package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // GCP auth lib for GKE

	targetBrowser "github.com/trilioData/tvk-plugins/tools/target-browser"
)

func init() {
	getCmd.AddCommand(backupCmd())

}

// backupCmd represents the backup command
func backupCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Aliases: []string{backupCmdPluralName},
		Use:     BackupCmdName,
		Short:   backupShortUsage,
		Long:    backupLongUsage,
		RunE:    getBackupList,
	}

	cmd.Flags().StringVarP(&backupPlanUID, BackupPlanUIDFlag, backupPlanUIDShort, backupPlanUIDDefault, backupPlanUIDUsage)
	cmd.Flags().StringVarP(&backupStatus, BackupStatusFlag, backupStatusShort, backupStatusDefault, backupStatusUsage)
	err := cmd.MarkFlagRequired(BackupPlanUIDFlag)
	if err != nil {
		log.Fatalf("Invalid option or missing required flag %s and Error is %s", BackupPlanUIDFlag, err.Error())
	}
	return cmd
}

func getBackupList(*cobra.Command, []string) error {

	bpOptions := targetBrowser.BackupListOptions{
		Page:          TargetBrowser.Pages,
		PageSize:      TargetBrowser.PageSize,
		Ordering:      TargetBrowser.OrderBy,
		BackupPlanUID: backupPlanUID,
		BackupStatus:  backupStatus,
	}
	err := targetBrowserAuthConfig.GetBackups(&bpOptions)
	if err != nil {
		return err
	}
	return nil
}
