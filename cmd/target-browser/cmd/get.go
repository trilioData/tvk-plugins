package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/trilioData/tvk-plugins/internal"
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
			Page:           pages,
			PageSize:       pageSize,
			OrderBy:        orderBy,
			OutputFormat:   outputFormat,
			OperationScope: operationScope,
			TvkInstanceUID: tvkInstanceUID,
		}

		return nil
	},
}

func init() {
	getCmd.PersistentFlags().IntVar(&pages, pagesFlag, pagesDefault, pagesUsage)
	getCmd.PersistentFlags().IntVar(&pageSize, PageSizeFlag, PageSizeDefault, pageSizeUsage)
	getCmd.PersistentFlags().StringVar(&orderBy, OrderByFlag, orderByDefault, orderByUsage)
	getCmd.PersistentFlags().StringVar(&operationScope, OperationScopeFlag, "", operationScopeUsage)
	getCmd.PersistentFlags().StringVar(&tvkInstanceUID, TvkInstanceUIDFlag, "", tvkInstanceUIDUsage)
	rootCmd.AddCommand(getCmd)
}

func validateInput(cmd *cobra.Command) error {
	if targetBrowserConfig.TargetName == "" {
		return fmt.Errorf("[%s] flag value cannot be empty", TargetNameFlag)
	}

	if cmd.Flags().Changed(CertificateAuthorityFlag) && targetBrowserConfig.CaCert == "" {
		return fmt.Errorf("[%s] flag value cannot be empty", CertificateAuthorityFlag)
	}

	if targetBrowserConfig.CaCert != "" && targetBrowserConfig.InsecureSkipTLS {
		return fmt.Errorf("[%s] flag cannot be provided if [%s] is provided",
			InsecureSkipTLSFlag, CertificateAuthorityFlag)
	}

	if outputFormat != "" && !internal.AllowedOutputFormats.Has(outputFormat) {
		return fmt.Errorf("[%s] flag invalid value. Usage - %s", OutputFormatFlag, OutputFormatFlagUsage)
	}

	if operationScope != "" {
		if strings.EqualFold(operationScope, internal.SingleNamespace) {
			operationScope = internal.SingleNamespace
		} else if strings.EqualFold(operationScope, internal.MultiNamespace) {
			operationScope = internal.MultiNamespace
		} else {
			return fmt.Errorf("[%s] flag invalid value. Usage - %s", OperationScopeFlag, operationScopeUsage)
		}
	}
	return nil
}
