package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	targetBrowser "github.com/trilioData/tvk-plugins/tools/targetbrowser"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // GCP auth lib for GKE
)

func init() {
	getCmd.AddCommand(backupCmd())

}

// backupCmd represents the backup command
func backupCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Aliases: []string{backupCmdPluralName},
		Use:     backupCmdName,
		Short:   shortUsage,
		Long:    longUsage,
		RunE:    runBackup,
	}

	cmd.Flags().IntVarP(&pageSize, pageSizeFlag, pageSizeShort, pageSizeDefault, pageSizeUsage)
	cmd.Flags().IntVarP(&page, pageFlag, pageShort, pageDefault, pageUsage)
	cmd.Flags().StringVarP(&ordering, orderingFlag, orderingShort, orderingDefault, orderingUsage)
	cmd.Flags().StringVarP(&backupPlanUID, backupPlanUIDFlag, backupPlanUIDShort, backupPlanUIDDefault, backupPlanUIDUsage)
	cmd.Flags().StringVarP(&backupStatus, backupStatusFlag, backupStatusShort, backupStatusDefault, backupStatusUsage)
	err := cmd.MarkFlagRequired(backupPlanUIDFlag)
	if err != nil {
		return nil
	}
	return cmd
}

func runBackup(*cobra.Command, []string) error {
	log.Info("---------    BackupPlan List start     --------- ")

	bpOptions := targetBrowser.BackupListOptions{
		Page:          page,
		PageSize:      pageSize,
		Ordering:      ordering,
		BackupPlanUID: backupPlanUID,
		BackupStatus:  backupStatus,
	}
	err := targetBrowser.NewClient(APIKey).GetBackups(&bpOptions)
	if err != nil {
		log.Fatalf("backup failed - %s", err.Error())
	}
	log.Info("---------    Backup List end   --------- ")
	return nil
}
