package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // GCP auth lib for GKE

	targetBrowser "github.com/trilioData/tvk-plugins/tools/targetbrowser"
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
		RunE:  runMetadata,
	}

	cmd.Flags().StringVarP(&backupPlanUID, backupPlanUIDFlag, backupPlanUIDShort, backupPlanUIDDefault, backupPlanUIDUsage)
	cmd.Flags().StringVarP(&backupUID, backupUIDFlag, backupUIDShort, backupUIDDefault, backupUIDUsage)
	err := cmd.MarkFlagRequired(backupPlanUIDFlag)
	if err != nil {
		log.Fatalf("Invalid option or missing required flag %s and Error is %s", backupPlanUIDFlag, err.Error())
		return nil
	}
	err = cmd.MarkFlagRequired(backupUIDFlag)
	if err != nil {
		log.Fatalf("Invalid option or missing required flag %s and Error is %s", backupUIDFlag, err.Error())
		return nil
	}
	return cmd
}

func runMetadata(*cobra.Command, []string) error {

	mdOptions := targetBrowser.MetadataListOptions{
		BackupPlanUID: backupPlanUID,
		BackupUID:     backupUID,
	}
	err := targetBrowser.NewClient(APIKey).GetMetadata(&mdOptions)
	if err != nil {
		return err
	}
	return nil

}
