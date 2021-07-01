package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	targetbrowser "github.com/trilioData/tvk-plugins/tools/target-browser"
)

var (
	targetBrowserAuthConfig = &targetbrowser.AuthInfo{}
	commonOptions           = targetbrowser.CommonListOptions{}
)

// nolint:lll // ignore long line lint errors
// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "get command retrieves specific resource",
	Long: `Performs GET operation on target-browser's '/backup', '/backupPlan' and '/metadata' APIs and gets specific resource [backup, backupPlan, metadata, etc]
which retrieves single object or list of objects of that resource.`,

	Example: `  # List of backups
  kubectl tvk-target-browser get backup --backup-plan-uid <uid> --target-name <name> --target-namespace <namespace>

  # List of backupPlans
  kubectl tvk-target-browser get backupPlan --target-name <name> --target-namespace <namespace>

  # Metadata of specific backup object
  kubectl tvk-target-browser get metadata --backup-uid <uid> --backup-plan-uid <uid> --target-name <name> --target-namespace <namespace>

  # Specific metadata
  kubectl tvk-target-browser get metadata --backup-uid <uid> --backup-plan-uid <uid> --target-name <name> --target-namespace <namespace>
`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		err = validateInput(cmd)
		if err != nil {
			return err
		}

		if targetBrowserAuthConfig, err = targetBrowserConfig.Authenticate(context.Background()); err != nil {
			return err
		}

		commonOptions = targetbrowser.CommonListOptions{
			Page:     pages,
			PageSize: pageSize,
			OrderBy:  orderBy,
		}

		return nil
	},
}

func init() {
	getCmd.PersistentFlags().IntVar(&pages, pagesFlag, pagesDefault, pagesUsage)
	getCmd.PersistentFlags().IntVar(&pageSize, PageSizeFlag, pageSizeDefault, pageSizeUsage)
	getCmd.PersistentFlags().StringVar(&orderBy, OrderByFlag, orderByDefault, orderByUsage)
	rootCmd.AddCommand(getCmd)
}

func validateInput(cmd *cobra.Command) error {
	if targetBrowserConfig.TargetName == "" {
		return fmt.Errorf("[%s] flag value cannot be empty", TargetNameFlag)
	}

	if cmd.Flags().Changed(certificateAuthorityFlag) && targetBrowserConfig.CaCert == "" {
		return fmt.Errorf("[%s] flag value cannot be empty", certificateAuthorityFlag)
	}

	if targetBrowserConfig.CaCert != "" && targetBrowserConfig.InsecureSkipTLS {
		return fmt.Errorf("[%s] flag cannot be provided if [%s] is provided",
			insecureSkipTLSFlag, certificateAuthorityFlag)
	}

	return nil
}
