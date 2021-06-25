package cmd

import (
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // GCP auth lib for GKE

	targetBrowser "github.com/trilioData/tvk-plugins/tools/target-browser"
)

func init() {
	getCmd.AddCommand(backupPlanCmd())
}

func backupPlanCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     BackupPlanCmdName,
		Aliases: []string{backupPlanCmdPluralName, backupPlanCmdAlias, backupPlanCmdAliasPlural},
		Short:   backupPlanShortUsage,
		Long:    backupPlanLongUsage,
		RunE:    getBackupPlanList,
	}

	cmd.Flags().StringVarP(&tvkInstanceUID, TvkInstanceUIDFlag, tvkInstanceUIDShort, tvkInstanceUIDDefault, tvkInstanceUIDUsage)
	return cmd
}

func getBackupPlanList(*cobra.Command, []string) error {

	bpOptions := targetBrowser.BackupPlanListOptions{
		Page:           TargetBrowser.Pages,
		PageSize:       TargetBrowser.PageSize,
		Ordering:       TargetBrowser.OrderBy,
		TvkInstanceUID: tvkInstanceUID,
	}
	err := targetBrowserAuthConfig.GetBackupPlans(&bpOptions)
	if err != nil {
		return err
	}
	return nil
}
