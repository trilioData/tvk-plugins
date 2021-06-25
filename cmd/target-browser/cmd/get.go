package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	targetbrowser "github.com/trilioData/tvk-plugins/tools/target-browser"
)

var targetBrowserAuthConfig = &targetbrowser.AuthInfo{}

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   GetFlag,
	Short: "get command retrieves specific resource",
	Long: `Gets specific resource[backup, backupPlan, metadata, etc] which retrieves single
object or list of objects of that resource.
`,

	Example: `  # List of backupPlans
    kubectl tvk-target-browser get backupPlan

    # List of backups
	kubectl target-browser get backup --backup-uid <uid>


	# List of backups
	kubectl tvk-target-browser get backup --backupplan-uid <uid>

	# Metadata of specific backup object

	kubectl tvk-target-browser get metadata --backup-uid <uid> --backupplan-uid <uid>

	kubectl target-browser get metadata

	# Specific metadata
	kubectl target-browser get metadata --backup-uid <uid> --backupplan-uid <uid>`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		err = validateInput(cmd)
		if err != nil {
			return err
		}

		if targetBrowserAuthConfig, err = TargetBrowser.Authenticate(context.Background()); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	getCmd.PersistentFlags().IntVar(&TargetBrowser.Pages, PagesFlag, pagesDefault, pagesUsage)
	getCmd.PersistentFlags().IntVar(&TargetBrowser.PageSize, PageSizeFlag, pageSizeDefault, pageSizeUsage)
	getCmd.PersistentFlags().StringVar(&TargetBrowser.OrderBy, OrderByFlag, orderByDefault, orderByUsage)

	rootCmd.AddCommand(getCmd)
}

func validateInput(cmd *cobra.Command) error {
	if TargetBrowser.TargetName == "" {
		return fmt.Errorf("[%s] flag value cannot be empty", TargetNameFlag)
	}

	if cmd.Flags().Changed(certificateAuthorityFlag) && TargetBrowser.CaCert == "" {
		return fmt.Errorf("[%s] flag value cannot be empty", certificateAuthorityFlag)
	}

	if cmd.Flags().Changed(clientCertificateFlag) && TargetBrowser.ClientCert == "" {
		return fmt.Errorf("[%s] flag value cannot be empty", clientCertificateFlag)
	}

	if cmd.Flags().Changed(clientKeyFlag) && TargetBrowser.ClientKey == "" {
		return fmt.Errorf("[%s] flag value cannot be empty", clientKeyFlag)
	}

	if TargetBrowser.CaCert != "" && TargetBrowser.InsecureSkipTLS {
		return fmt.Errorf("[%s] flag cannot be provided if [%s] is provided",
			insecureSkipTLSFlag, certificateAuthorityFlag)
	}

	if (TargetBrowser.ClientKey != "" && TargetBrowser.ClientCert == "") ||
		(TargetBrowser.ClientKey == "" && TargetBrowser.ClientCert != "") {
		return fmt.Errorf("both [%s] flag and [%s] flag needs to be provided to use TLS transport",
			clientKeyFlag, clientCertificateFlag)
	}

	return nil
}
