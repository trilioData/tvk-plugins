package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	targetBrowser "github.com/trilioData/tvk-plugins/tools/target-browser"
)

func init() {
	getCmd.AddCommand(resourceMetadataCmd())
}

// resourceMetadataCmd represents the resource-metadata command
func resourceMetadataCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   ResourceMetadataCmdName,
		Short: "Get Backup high level metadata details",
		Long:  `Performs GET operation on target-browser's '/resource-metadata' API and gets Application YAMLs from backup in JSON format`,
		Example: `  # Get resource-metadata details of specific backup
  kubectl tvk-target-browser get resource-metadata --backup-uid <uid> --backup-plan-uid <uid> --name <name> --group <group> 
	--version <version> --kind <kind>
`,
		RunE: getResourceMetadata,
	}

	cmd.Flags().StringVar(&group, groupFlag, groupDefault, groupUsage)

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

	cmd.Flags().StringVar(&version, versionFlag, versionDefault, versionUsage)
	err = cmd.MarkFlagRequired(versionFlag)
	if err != nil {
		log.Fatalf("Invalid option or missing required flag %s - %s", versionFlag, err.Error())
	}

	cmd.Flags().StringVar(&kind, kindFlag, kindDefault, kindUsage)
	err = cmd.MarkFlagRequired(kindFlag)
	if err != nil {
		log.Fatalf("Invalid option or missing required flag %s - %s", kindFlag, err.Error())
	}

	cmd.Flags().StringVar(&name, nameFlag, nameDefault, nameUsage)
	err = cmd.MarkFlagRequired(nameFlag)
	if err != nil {
		log.Fatalf("Invalid option or missing required flag %s - %s", nameFlag, err.Error())
	}

	return cmd
}

func getResourceMetadata(*cobra.Command, []string) error {
	resourceMdOptions := targetBrowser.ResourceMetadataListOptions{
		BackupPlanUID: backupPlanUID,
		BackupUID:     backupUID,
		Name:          name,
		Group:         group,
		Kind:          kind,
		Version:       version,
		OutputFormat:  outputFormat,
	}

	err := targetBrowserAuthConfig.GetResourceMetadata(&resourceMdOptions)
	if err != nil {
		return err
	}
	return nil

}
