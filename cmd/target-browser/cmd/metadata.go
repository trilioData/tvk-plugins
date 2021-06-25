package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // GCP auth lib for GKE

	targetBrowser "github.com/trilioData/tvk-plugins/tools/target-browser"
)

func init() {
	getCmd.AddCommand(metadataCmd())
}

// metadataCmd represents the metadata command
func metadataCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   metadataCmdName,
		Short: metadataShortUsage,
		Long:  metadataLongUsage,
		RunE:  getMetadata,
	}

	cmd.Flags().StringVarP(&backupPlanUID, BackupPlanUIDFlag, backupPlanUIDShort, backupPlanUIDDefault, backupPlanUIDUsage)
	cmd.Flags().StringVarP(&backupUID, BackupUIDFlag, backupUIDShort, backupUIDDefault, backupUIDUsage)
	err := cmd.MarkFlagRequired(BackupPlanUIDFlag)
	if err != nil {
		log.Fatalf("Invalid option or missing required flag %s and Error is %s", BackupPlanUIDFlag, err.Error())
	}
	err = cmd.MarkFlagRequired(BackupUIDFlag)
	if err != nil {
		log.Fatalf("Invalid option or missing required flag %s and Error is %s", BackupUIDFlag, err.Error())
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
