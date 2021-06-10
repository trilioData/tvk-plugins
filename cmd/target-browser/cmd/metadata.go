package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	targetBrowser "github.com/trilioData/tvk-plugins/tools/targetbrowser"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // GCP auth lib for GKE
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
		return nil
	}
	err = cmd.MarkFlagRequired(backupUIDFlag)
	if err != nil {
		return nil
	}
	return cmd
}

func runMetadata(*cobra.Command, []string) error {
	log.Info("---------    Metadata List start   --------- ")

	mdOptions := targetBrowser.MetadataListOptions{
		BackupPlanUID: backupPlanUID,
		BackupUID:     backupUID,
	}
	err := targetBrowser.NewClient(APIKey).GetMetadata(&mdOptions)
	if err != nil {
		log.Fatalf("metadata failed - %s", err.Error())
	}
	log.Info("---------  metadata List end   --------- ")
	return nil

}
