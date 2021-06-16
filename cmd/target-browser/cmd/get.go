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
	Use:   "get",
	Short: "get command retrieves specific resource",
	Long: `Gets specific resource[backup, backupPlan, metadata, etc] which retrieves single
object or list of objects of that resource.
`,
	Example: `  # List of backups	
  kubectl target-browser get backup

  # List of backupPlans
  kubectl target-browser get backupPlan

  # Metadata of specific backup object
  kubectl target-browser get metadata

  # Specific backup
  kubectl target-browser get backup --backup-uid <uid> --backupplan-uid <uid>
`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		err = validateInput(cmd)
		if err != nil {
			return err
		}

		if targetBrowserAuthConfig, err = TargetBrowser.Authenticate(context.Background()); err != nil {
			return err
		}
		// TODO: remove this line and use 'targetBrowserAuthConfig' as a receiver for getCmd's child commands
		fmt.Println(targetBrowserAuthConfig)
		return nil
	},
}

func init() {
	getCmd.PersistentFlags().IntVar(&TargetBrowser.Pages, pagesFlag, pagesDefault, pagesUsage)
	getCmd.PersistentFlags().IntVar(&TargetBrowser.PageSize, pageSizeFlag, pageSizeDefault, pageSizeUsage)
	getCmd.PersistentFlags().StringVar(&TargetBrowser.OrderBy, orderByFlag, orderByDefault, orderByUsage)

	rootCmd.AddCommand(getCmd)
}

func validateInput(cmd *cobra.Command) error {
	if TargetBrowser.TargetName == "" {
		return fmt.Errorf("[%s] flag value cannot be empty", targetNameFlag)
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
