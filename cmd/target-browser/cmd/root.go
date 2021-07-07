package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/trilioData/tvk-plugins/internal"
	targetbrowser "github.com/trilioData/tvk-plugins/tools/target-browser"
)

// nolint:lll // ignore long line lint errors
// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "target-browser",
	Short: "target-browser cli utility queries content of mounted target",
	Long: `target-browser cli utility can query content of mounted target location to get details of backup, backupPlan and metadata details of backup
via HTTP/HTTPS calls to target-browser server.
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
	scheme              = runtime.NewScheme()
	targetBrowserConfig = &targetbrowser.Config{}
)

func init() {
	rootCmd.PersistentFlags().StringVar(&targetBrowserConfig.KubeConfig, KubeConfigFlag, internal.KubeConfigDefault, kubeConfigUsage)
	rootCmd.PersistentFlags().BoolVar(&targetBrowserConfig.InsecureSkipTLS, InsecureSkipTLSFlag, false, insecureSkipTLSUsage)
	rootCmd.PersistentFlags().StringVar(&targetBrowserConfig.CaCert, CertificateAuthorityFlag, "", certificateAuthorityUsage)
	rootCmd.PersistentFlags().BoolVar(&targetBrowserConfig.UseHTTPS, UseHTTPS, false, useHTTPSUsage)

	rootCmd.PersistentFlags().StringVar(&targetBrowserConfig.TargetNamespace, TargetNamespaceFlag,
		targetNamespaceDefault, targetNamespaceUsage)
	rootCmd.PersistentFlags().StringVar(&targetBrowserConfig.TargetName, TargetNameFlag, "", targetNameUsage)
	if err := rootCmd.MarkPersistentFlagRequired(TargetNameFlag); err != nil {
		log.Fatalf("failed to mark flag %s as required - %s", TargetNameFlag, err.Error())
	}

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1beta1.AddToScheme(scheme))
	targetBrowserConfig.Scheme = scheme
}
