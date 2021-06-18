package cmd

import (
	"log"

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/spf13/cobra"

	"github.com/trilioData/tvk-plugins/internal"
	targetbrowser "github.com/trilioData/tvk-plugins/tools/target-browser"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "target-browser",
	Short: "target-browser cli utility queries content of mounted target",
	Long: `target-browser cli utility can query content of mounted target location to get details of backup, backupPlan and
metadata details of backup via HTTP/HTTPS calls to target-browser server.
`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("target-browser command execution failed - %s", err.Error())
	}
}

var (
	scheme        = runtime.NewScheme()
	TargetBrowser = &targetbrowser.Config{}
)

func init() {
	rootCmd.PersistentFlags().StringVar(&TargetBrowser.KubeConfig, kubeConfigFlag, internal.KubeConfigDefault, kubeConfigUsage)
	rootCmd.PersistentFlags().BoolVar(&TargetBrowser.InsecureSkipTLS, insecureSkipTLSFlag, false, insecureSkipTLSUsage)
	rootCmd.PersistentFlags().StringVar(&TargetBrowser.CaCert, certificateAuthorityFlag, "", certificateAuthorityUsage)
	rootCmd.PersistentFlags().StringVar(&TargetBrowser.ClientCert, clientCertificateFlag, "", clientCertificateUsage)
	rootCmd.PersistentFlags().StringVar(&TargetBrowser.ClientKey, clientKeyFlag, "", clientKeyUsage)

	rootCmd.PersistentFlags().StringVar(&TargetBrowser.TargetNamespace, targetNamespaceFlag, targetNamespaceDefault, targetNamespaceUsage)
	rootCmd.PersistentFlags().StringVar(&TargetBrowser.TargetName, targetNameFlag, "", targetNameUsage)
	if err := rootCmd.MarkPersistentFlagRequired(targetNameFlag); err != nil {
		log.Fatalf("failed to mark flag %s as required - %s", targetNameFlag, err.Error())
	}

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1beta1.AddToScheme(scheme))
	TargetBrowser.Scheme = scheme
}
