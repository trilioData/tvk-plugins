package targetbrowsertest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"github.com/thedevsaddam/gojsonq"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientGoScheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"

	"github.com/trilioData/tvk-plugins/cmd/target-browser/cmd"
	"github.com/trilioData/tvk-plugins/internal"
	"github.com/trilioData/tvk-plugins/internal/utils/shell"
	testutils "github.com/trilioData/tvk-plugins/tests/test_utils"
	targetbrowser "github.com/trilioData/tvk-plugins/tools/target-browser"
)

var (
	k8sClient client.Client
	ctx       = context.Background()
	installNs = testutils.GetInstallNamespace()

	controlPlaneDeploymentKey = types.NamespacedName{
		Name:      internal.TVKControlPlaneDeployment,
		Namespace: installNs,
	}

	createBackupScript = "./createBackups.sh"

	targetYaml                  = "target.yaml"
	tlsKeyFile                  = "tls.key"
	tlsCertFile                 = "tls.crt"
	tvkIngName                  = "k8s-triliovault"
	tlsSecretName               = "ssl-certs"
	nfsIPAddr                   string
	nfsServerPath               string
	currentDir, _               = os.Getwd()
	projectRoot                 = filepath.Dir(filepath.Dir(currentDir))
	testDataDirRelPath          = filepath.Join(projectRoot, "tests", "target-browser", "test-data")
	targetBrowserBinaryDir      = filepath.Join(projectRoot, DistDir, TargetBrowserDir)
	targetBrowserBinaryFilePath = filepath.Join(targetBrowserBinaryDir, TargetBrowserBinaryName)
	targetYamlPath              = filepath.Join(testDataDirRelPath, targetYaml)
)

const sampleTVKHost = "k8s-tvk.com"

func TestTargetBrowser(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("target-browser-junit.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "TargetBrowser Suite", []Reporter{junitReporter})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")

	scheme := runtime.NewScheme()
	_ = clientGoScheme.AddToScheme(scheme)

	kubeConfig, err := internal.NewConfigFromCommandline("")
	Expect(err).Should(BeNil())

	acc, err := internal.NewAccessor(kubeConfig, scheme)
	Expect(err).Should(BeNil())

	k8sClient = acc.GetRuntimeClient()
	Expect(k8sClient).ToNot(BeNil())

	Expect(os.Setenv(NFSServerBasePath, TargetBrowserDataPath)).To(BeNil())

	_, err = shell.Mkdir(TargetLocation)
	Expect(err).Should(BeNil())

	log.Info("Mounting target.")
	mountTarget()
	changeControlPlanePollingPeriod()
	time.Sleep(time.Second * 10)

	makeRandomDirAndMount()
	nfsIPAddr, nfsServerPath = getNFSCredentials()
	Expect(updateYAMLs(
		map[string]string{
			NFSServerIP:       nfsIPAddr,
			NFSServerBasePath: nfsServerPath,
		}, filepath.Join(testDataDirRelPath, targetYaml))).To(BeNil())

	// Update tvk host
	updateTvkHostIngress()
}, 60)

var _ = AfterSuite(func() {
	Expect(updateYAMLs(
		map[string]string{
			nfsIPAddr:     NFSServerIP,
			nfsServerPath: NFSServerBasePath,
		}, filepath.Join(testDataDirRelPath, targetYaml))).To(BeNil())
	removeRandomDirAndUnmount()
})

func changeControlPlanePollingPeriod() {

	var (
		container    *corev1.Container
		containerIdx int
		// setting polling period to update browser cache to 10 seconds
		pollingPeriodTime = "10s"
	)

	By("Getting Control Plane Deployment")
	deployment := &appsv1.Deployment{}
	err := k8sClient.Get(ctx, controlPlaneDeploymentKey, deployment)
	Expect(err).To(BeNil())
	containers := deployment.Spec.Template.Spec.Containers
	for index := range containers {
		if containers[index].Name == ControlPlaneContainerName {
			container = &containers[index]
			containerIdx = index
			break
		}
	}
	if container != nil {
		for index := range container.Env {
			if container.Env[index].Name == PollingPeriod {
				container.Env[index].Value = pollingPeriodTime
				deployment.Spec.Template.Spec.Containers[containerIdx].Env = container.Env
				break
			}
		}
	}

	Eventually(func() error {
		err = k8sClient.Update(ctx, deployment)
		return err
	}, timeout, interval).Should(BeNil())

	Expect(err).ShouldNot(HaveOccurred())
}

func createTarget(enableBrowsing bool) {

	if !enableBrowsing {
		Expect(updateYAMLs(map[string]string{"enableBrowsing: true": "enableBrowsing: false"}, targetYamlPath)).To(BeNil())
	}

	log.Infof("Creating target with enableBrowsing=%v and waiting for it to become available", enableBrowsing)
	targetCmd := fmt.Sprintf("kubectl apply -f %s --namespace %s", targetYamlPath, installNs)
	command := exec.Command("bash", "-c", targetCmd)

	output, err := command.CombinedOutput()
	if err != nil {
		Fail(fmt.Sprintf("target creation failed %s - %s", err.Error(), string(output)))
	}

	verifyTargetStatus(ctx, installNs, k8sClient)
	if enableBrowsing {
		verifyTargetBrowsingEnabled(ctx, installNs, k8sClient)
	}
	log.Infof("Created target %s successfully", TargetName)
}

func deleteTarget(enableBrowsing bool) {
	if enableBrowsing {
		Expect(updateYAMLs(map[string]string{"enableBrowsing: false": "enableBrowsing: true"}, targetYamlPath)).To(BeNil())
	}
	targetCmd := fmt.Sprintf("kubectl delete -f %s --namespace %s", filepath.Join(testDataDirRelPath, targetYaml), installNs)
	command := exec.Command("bash", "-c", targetCmd)
	out, err := command.CombinedOutput()
	log.Error(string(out))
	if err != nil {
		Fail(fmt.Sprintf("target deletion failed %s.", err.Error()))
	}
	checkPvcDeleted(ctx, k8sClient, installNs)
	log.Infof("Deleted target %s successfully", TargetName)
}
func mergeArgs(args []string) (string, []string) {
	var opFormat string
	for idx, val := range args {
		if val == flagOutputFormat && len(args) > idx+1 {
			opFormat = args[idx+1]
			break
		}
	}
	args = append(args, commonArgs...)
	return opFormat, args
}

func runCmdBackupPlan(args []string) []targetbrowser.BackupPlan {
	var (
		backupPlanList targetbrowser.BackupPlanList
	)
	respBytes := exeCommand(args, cmdBackupPlan)

	err := json.Unmarshal(respBytes.Bytes(), &backupPlanList.Results)
	if err != nil {
		Fail(fmt.Sprintf("Failed to unmarshal backupplan command's data %s", err.Error()))
	}
	return backupPlanList.Results
}

func exeCommand(args []string, cmdName string) bytes.Buffer {
	var (
		output   []byte
		opFormat string
		selector = targetbrowser.BackupPlanSelector
		err      error
	)

	opFormat, args = mergeArgs(args)
	Eventually(func() bool {
		command := exec.Command(targetBrowserBinaryFilePath, args...)
		log.Infof("%s command is: %s", cmdName, command)
		output, err = command.CombinedOutput()
		if err != nil {
			log.Errorf(fmt.Sprintf("Error to execute command %s", err.Error()))
			log.Infof("%s data is %s", cmdName, output)
		}
		return strings.Contains(string(output), "502 Bad Gateway")
	}, apiRetryTimeout, interval).Should(BeFalse())

	if cmdName == cmdBackup {
		selector = targetbrowser.BackupSelector
	}
	if opFormat == internal.FormatYAML {
		output, err = yaml.YAMLToJSON(output)
		Expect(err).To(BeNil())
	} else if opFormat == internal.FormatWIDE || opFormat == "" {
		output = convertTSVToJSON(output)
		selector = backupPlanSelector
		if cmdName == cmdBackup {
			selector = backupSelector
		}
	}
	return formatOutput(string(output), selector)
}

func formatOutput(finalOutput string, selector []string) bytes.Buffer {
	var (
		err       error
		respBytes bytes.Buffer
	)
	Eventually(func() error {
		if len(finalOutput) == 0 {
			return nil
		}
		jsq := gojsonq.New().FromString(finalOutput).From(internal.Results).Select(selector...)
		if err = jsq.Error(); err != nil {
			log.Warn(err.Error())
			if strings.Contains(err.Error(), "looking for beginning of value") {
				slicedStrings := strings.SplitAfter(finalOutput, "\n")
				finalOutput = strings.Join(slicedStrings[1:], "\n")
				return err
			}
			Fail(err.Error())
		}

		jsq.Writer(&respBytes)
		return nil
	}, time.Second*30, interval).Should(BeNil())
	return respBytes
}

func runCmdBackup(args []string) []targetbrowser.Backup {
	var (
		err        error
		backupList targetbrowser.BackupList
	)

	respBytes := exeCommand(args, cmdBackup)
	err = json.Unmarshal(respBytes.Bytes(), &backupList.Results)
	if err != nil {
		Fail(fmt.Sprintf("Failed to unmarshal backup command's output %s.", err.Error()))
	}
	return backupList.Results
}

func switchTvkHostFromHTTPToHTTPS() {
	//create tls secret
	createTLSSecret(tlsSecretName)

	//patch ingress with tls config
	ing := GetIngress(ctx, k8sClient, tvkIngName, installNs)

	tlsConfig := v1beta1.IngressTLS{SecretName: tlsSecretName, Hosts: []string{ing.Spec.Rules[0].Host}}
	ing.Spec.TLS = append(ing.Spec.TLS, tlsConfig)
	UpdateIngress(ctx, k8sClient, ing)
	log.Info("Successfully switched TVK host from HTTP to HTTPS")
}

func switchTvkHostFromHTTPSToHTTP() {
	//delete tls secret
	secret := GetSecret(ctx, k8sClient, tlsSecretName, installNs)
	log.Info("delete secret")
	Expect(k8sClient.Delete(ctx, secret)).To(BeNil())

	//patch ingress and remove tls config
	ing := GetIngress(ctx, k8sClient, tvkIngName, installNs)
	ing.Spec.TLS = []v1beta1.IngressTLS{}
	UpdateIngress(ctx, k8sClient, ing)
	log.Info("Successfully switched TVK host from HTTPS to HTTP")
}

func createTLSSecret(secretName string) {
	tlsKeyBytes, err := ioutil.ReadFile(filepath.Join(testDataDirRelPath, tlsKeyFile))
	Expect(err).To(BeNil())

	tlsCertBytes, err := ioutil.ReadFile(filepath.Join(testDataDirRelPath, tlsCertFile))
	Expect(err).To(BeNil())

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: installNs},
		Type:       corev1.SecretTypeTLS,
		StringData: map[string]string{
			"tls.crt": string(tlsCertBytes),
			"tls.key": string(tlsKeyBytes)},
	}
	Expect(k8sClient.Create(ctx, secret)).To(BeNil())
	log.Infof("created TLS type secret %s", secretName)
}

func getTargetBrowserIngress() *v1beta1.Ingress {

	target := getTarget(ctx, installNs, k8sClient)

	ingressList := v1beta1.IngressList{}
	err := k8sClient.List(ctx, &ingressList, client.InNamespace(installNs))
	Expect(err).To(BeNil())

	for i := range ingressList.Items {
		ing := ingressList.Items[i]
		ownerRefs := ing.GetOwnerReferences()
		for j := range ownerRefs {
			ownerRef := ownerRefs[j]
			if ownerRef.Kind == target.GetKind() && ownerRef.UID == target.GetUID() {
				return &ing
			}
		}
	}

	return nil
}

func updateTvkHostIngress() {
	ing := GetIngress(ctx, k8sClient, tvkIngName, installNs)
	Expect(len(ing.Spec.Rules)).Should(BeNumerically(">", 0))
	ing.Spec.Rules[0].Host = fmt.Sprintf("%s.%s", installNs, sampleTVKHost)
	UpdateIngress(ctx, k8sClient, ing)
}

func verifyBrowserCacheBPlan(noOfBackupPlan int) bool {
	args := []string{cmdGet, cmdBackupPlan, flagOutputFormat, internal.FormatJSON}
	var backupPlanData []targetbrowser.BackupPlan
	Eventually(func() bool {
		backupPlanData = runCmdBackupPlan(args)
		return len(backupPlanData) == noOfBackupPlan || len(backupPlanData) == cmd.PageSizeDefault
	}, timeout, apiRetryTimeout).Should(BeTrue())

	return len(backupPlanData) == noOfBackupPlan || len(backupPlanData) == cmd.PageSizeDefault
}

func createBackups(noOfBackupPlansToCreate, noOfBackupsToCreate int, backupUID, backupType string, extraParams ...string) {
	output, err := exec.Command(createBackupScript, strconv.Itoa(noOfBackupPlansToCreate),
		strconv.Itoa(noOfBackupsToCreate), backupType, backupUID, strings.Join(extraParams, ",")).Output()
	Expect(err).To(BeNil())
	log.Info("Shell Script Output: ", string(output))
}
