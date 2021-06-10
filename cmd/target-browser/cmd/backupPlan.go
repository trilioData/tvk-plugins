package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	targetBrowser "github.com/trilioData/tvk-plugins/tools/targetbrowser"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"    // GCP auth lib for GKE

)

func init() {
	getCmd.AddCommand(backupPlanCmd())
}

func backupPlanCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     backupPlanCmdName,
		Aliases: []string{backupPlanCmdPluralName, backupPlanCmdAlias, backupPlanCmdAliasPlural},
		Short: shortUsage,
		Long:  longUsage,
		RunE:  runBackupPlan,
	}

	cmd.Flags().IntVarP(&pageSize, pageSizeFlag, pageSizeShort, pageSizeDefault, pageSizeUsage)
	cmd.Flags().IntVarP(&page, pageFlag, pageShort, pageDefault, pageUsage)
	cmd.Flags().StringVarP(&ordering, orderingFlag, orderingShort, orderingDefault, orderingUsage)
	cmd.Flags().StringVarP(&tvkInstanceUID, tvkInstanceUIDFlag, tvkInstanceUIDShort, tvkInstanceUIDDefault, tvkInstanceUIDUsage)
	return cmd
}

func runBackupPlan(*cobra.Command, []string) error {
	log.Info("---------    BackupPlan List start  --------- ")

	bpOptions := targetBrowser.ListOptions{
		Page:           page,
		PageSize:       pageSize,
		Ordering:       ordering,
		TvkInstanceUID: tvkInstanceUID,
	}
	err := targetBrowser.NewClient(APIKey).GetBackupPlans(&bpOptions)
	if err != nil {
		log.Fatalf("backupplan failed - %s", err.Error())
	}
	log.Info("---------    BackupPlan List end   --------- ")
	return nil
}
