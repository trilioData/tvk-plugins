package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	targetBrowser "github.com/trilioData/tvk-plugins/tools/target-browser"
)

func init() {
	getCmd.AddCommand(metadataCmd())
}

// metadataCmd represents the metadata command
func metadataCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   MetadataCmdName,
		Short: "API to perform Read operations on Backup high level metadata",
		Long: `API to perform Read operations on Backup high level metadata.
  Get metadata of specific backup using flag backup-plan-uid and backup-uid`,

		Example: `  # Get Specific metadata of backup
  kubectl tvk-target-browser get metadata --backup-uid <uid> --backup-plan-uid <uid>`,
		RunE: getMetadata,
	}

	cmd.Flags().StringVar(&backupPlanUID, BackupPlanUIDFlag, backupPlanUIDDefault, backupPlanUIDUsage)
	err := cmd.MarkFlagRequired(BackupPlanUIDFlag)
	if err != nil {
		log.Fatalf("Invalid option or missing required flag %s - %s", BackupPlanUIDFlag, err.Error())
	}
	cmd.Flags().StringVar(&backupUID, BackupUIDFlag, backupUIDDefault, backupUIDUsage)
	err = cmd.MarkFlagRequired(BackupUIDFlag)
	if err != nil {
		log.Fatalf("Invalid option or missing required flag %s - %s", BackupUIDFlag, err.Error())
	}
	return cmd
}

func getMetadata(*cobra.Command, []string) error {

	mdOptions := targetBrowser.MetadataListOptions{
		BackupPlanUID: backupPlanUID,
		BackupUID:     backupUID,
	}

	err := targetBrowserAuthConfig.GetMetadata(&mdOptions)
	if err != nil {
		return err
	}
	return nil

}
