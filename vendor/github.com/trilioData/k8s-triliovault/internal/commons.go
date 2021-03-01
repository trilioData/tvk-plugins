package internal

import (
	"bytes"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	builtinruntime "runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/profefe/profefe/agent"

	crd "github.com/trilioData/k8s-triliovault/api/v1"

	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ActiveDeadlineSeconds     = int64(12 * 3600)
	MaxVolumeSnapshotDuration = 3 * time.Hour
	FailureBackoffLimit       = int32(1)
	NeverBackoffLimit         = int32(0)
	RetentionBackoffLimit     = int32(3)

	AppsResourcesMap = map[string]runtime.Object{
		DaemonSetKind:             &appsv1.DaemonSet{},
		DeploymentKind:            &appsv1.Deployment{},
		ReplicaSetKind:            &appsv1.ReplicaSet{},
		StatefulSetKind:           &appsv1.StatefulSet{},
		ReplicationControllerKind: &corev1.ReplicationController{},
	}

	BatchResourcesMap = map[string]runtime.Object{
		JobKind:     &batchv1.Job{},
		CronJobKind: &batchv1beta1.CronJob{},
	}

	DryRunResourcesMap = map[string]runtime.Object{
		ServiceKind: &corev1.Service{},
	}
)

type PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func GetClusterType() bool {
	cluster, present := os.LookupEnv(IsOpenshift)
	if !present {
		return false
	}
	if cluster == "true" {
		return true
	}
	return false
}

func GetInstallNamespace() string {
	namespace, present := os.LookupEnv(InstallNamespace)
	if !present {
		panic("Install Namespace not found in environment")
	}
	return namespace
}

func GetTrilioNamespace() string {
	if GetAppScope() == NamespacedScope {
		return GetInstallNamespace()
	}
	return ""
}

func GetTVKVersion() string {
	version, present := os.LookupEnv(TVKVersion)
	if !present {
		panic("TVK version not found in environment")
	}
	return version
}

func GetAppScope() string {
	scope, present := os.LookupEnv(AppScope)
	if !present {
		panic("App Scope not found in environment")
	}
	return scope
}

func GetTrilioResourcesDefaultListOpts() *client.ListOptions {
	opts := &client.ListOptions{}
	// For Namespace scoped installation Trilio resources only from installation namespace retrieved.
	if GetAppScope() == NamespacedScope {
		client.InNamespace(GetInstallNamespace()).ApplyToList(opts)
	}

	return opts
}

func GenerateRandomString(n int, isOnlyAlphabetic bool) string {
	letters := "abcdefghijklmnopqrstuvwxyz"
	numbers := "1234567890"
	rand.Seed(time.Now().UnixNano())
	letterRunes := []rune(letters)
	if !isOnlyAlphabetic {
		letterRunes = []rune(letters + numbers)
	}
	b := make([]rune, n)
	for i := range b {
		randNum, _ := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(len(letterRunes))))
		b[i] = letterRunes[randNum.Int64()]
	}
	return string(b)
}

func GenerateRandomInt(start, end int) (randInt int) {
	rand.Seed(time.Now().UnixNano())
	// nolint:gosec // no need to take bigInt random generator here
	randInt = rand.Intn(end) + start
	return randInt
}

func GenerateRandomBool() bool {
	rand.Seed(time.Now().UnixNano())
	return GenerateRandomInt(0, 10)%2 == 0
}

func GetPVCname(pvcMetadata string) (string, error) {
	pvcStruct, err := GetPVCStruct(pvcMetadata)
	if err != nil {
		return "", err
	}
	return pvcStruct.GetName(), nil
}

func GetPVCStruct(pvcMetadata string) (corev1.PersistentVolumeClaim, error) {
	pvcStruct := corev1.PersistentVolumeClaim{}
	err := json.Unmarshal([]byte(pvcMetadata), &pvcStruct)
	if err != nil {
		return pvcStruct, err
	}
	return pvcStruct, nil
}

func GetNestedString(obj map[string]interface{}, fields ...string) string {
	val, found, err := unstructured.NestedString(obj, fields...)
	if !found || err != nil {
		return ""
	}
	return val
}

func CheckRequiredFlags(flags *flag.FlagSet, requiredFlags ...string) error {
	required := map[string]struct{}{}
	for _, name := range requiredFlags {
		required[name] = struct{}{}
	}

	var unSetFlags []string

	flags.VisitAll(func(flag *flag.Flag) {
		_, flagRequired := required[flag.Name]

		if flagRequired && flag.Value.String() == "" {
			unSetFlags = append(unSetFlags, flag.Name)
		}
	})

	if len(unSetFlags) > 0 {
		return fmt.Errorf(`missing required flag(s): "%s"`, strings.Join(unSetFlags, `", "`))
	}

	return nil
}

func RemoveEmptyStringArgs(list []string) []string {
	var (
		tmp []string
	)
	for _, c := range list {
		if c != "" {
			tmp = append(tmp, c)
		}
	}
	return tmp
}

func GetGVKFromCRD(crdObj *unstructured.Unstructured) (crd.GroupVersionKind, error) {
	var retGVK crd.GroupVersionKind
	kind, kindFound, kindErr := unstructured.NestedString(crdObj.Object, "spec",
		"names", "kind")
	if !kindFound || kindErr != nil {
		return retGVK, kindErr
	}

	group, gFound, gErr := unstructured.NestedString(crdObj.Object, "spec",
		"group")
	if !gFound || gErr != nil {
		return retGVK, gErr
	}

	version, versionFound, versionErr := unstructured.NestedString(crdObj.Object, "spec", "version")
	if !versionFound || versionErr != nil {
		versions, versionsfound, versionsErr := unstructured.NestedSlice(crdObj.Object, "spec", "versions")
		if versionsErr != nil {
			return retGVK, versionsErr
		}
		if !versionsfound {
			return retGVK, fmt.Errorf("couldn't get the version from crd %s", crdObj)
		}

		for i := range versions {
			v := versions[i].(map[string]interface{})
			storage := v["storage"].(bool)
			if storage {
				version = v["name"].(string)
				break
			}
		}
	}
	retGVK.Group = group
	retGVK.Kind = kind
	retGVK.Version = version

	return retGVK, nil
}

func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func FileExists(filePath string) bool {
	// FileExists checks if file is present
	// Input:
	//		filePath: File Path
	// Output:
	//		True/False depending upon the file existence

	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}

	return !info.IsDir()
}

func DirExists(dirPath string) bool {
	// DirExists checks if directory is present
	// Input:
	//		dirPath: Directory Path
	// Output:
	//		True/False depending upon the directory existence

	info, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		return false
	}

	return info.IsDir()
}

func ReadChildDir(dirPath string) (dirNames []string, err error) {
	// ReadChildDir reads the dir name from the given path
	// Input:
	//		dirPath: Directory path
	// Output:
	//		dirNames: Directory name list
	//		err: Error

	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return dirNames, err
	}
	for _, file := range files {
		if file.IsDir() {
			dirNames = append(dirNames, file.Name())
		}
	}

	return dirNames, nil
}

func GetGVKFromString(gvk string) schema.GroupVersionKind {
	strs := strings.Split(gvk, Comma)
	apiVersion := strings.TrimSpace(strs[0])
	tmp := strings.Split(strs[1], Equals)
	kind := strings.TrimSpace(tmp[1])
	return schema.FromAPIVersionAndKind(apiVersion, kind)
}

func GetNodeAffinity() *corev1.Affinity {
	return &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "beta.kubernetes.io/arch",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"amd64"},
							},
						},
					}, {
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/arch",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"amd64"},
							}},
					}},
			},
		},
	}
}

func ValidateName(inputStr string, reqLength int) (outputStr string, isValid bool) {
	if len(inputStr) > reqLength {
		inputStr = inputStr[:reqLength]
	}

	outputStr = inputStr
	for i := 1; i < len(inputStr); i++ {
		if len(validation.IsQualifiedName(outputStr)) == 0 {
			isValid = true
			break
		} else {
			outputStr = inputStr[:len(outputStr)-1]
		}
	}
	return
}

func ValidateLabel(inputStr string, reqLength int) (outputStr string, isValid bool) {
	if len(inputStr) > reqLength {
		inputStr = inputStr[:reqLength]
	}

	outputStr = inputStr
	for i := 1; i < len(inputStr); i++ {
		if len(validation.IsValidLabelValue(outputStr)) == 0 {
			isValid = true
			break
		} else {
			outputStr = inputStr[:len(outputStr)-1]
		}
	}

	return
}

func GetRecommendedLabels(instanceLabel, managedBy string) map[string]string {
	validatedInstanceLabel, isILValid := ValidateLabel(instanceLabel, MaxNameOrLabelLen)
	if !isILValid {
		// fallback to default label
		validatedInstanceLabel = DefaultLabel
	}
	recommendedLabels := map[string]string{
		"app.kubernetes.io/name":       DefaultLabel,
		"app.kubernetes.io/instance":   validatedInstanceLabel,
		"app.kubernetes.io/managed-by": managedBy,
		"app.kubernetes.io/part-of":    PartOf,
	}

	return recommendedLabels
}

func GetLicensePublicKeyPath() string {
	filePath := "license_keys/triliodata.pub"
	if !FileExists(filePath) {
		panic("license public key not found")
	}
	return filePath
}

func GenerateTLSCerts(commonName string, dnsNames []string) (caPEM, serverCertPEM, serverPrivKeyPEM *bytes.Buffer) {

	// CA config
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2020),
		Subject: pkix.Name{
			Organization: []string{"Trilio.io"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// CA private key
	caPrivKey, err := rsa.GenerateKey(cryptorand.Reader, 4096)
	if err != nil {
		fmt.Println(err)
	}

	// Self signed CA certificate
	caBytes, err := x509.CreateCertificate(cryptorand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		fmt.Println(err)
	}

	// PEM encode CA cert
	caPEM = new(bytes.Buffer)
	_ = pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	// PEM encode CA private Key
	caPrivKeyPEM := new(bytes.Buffer)
	_ = pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})

	// server cert config
	cert := &x509.Certificate{
		DNSNames:     dnsNames,
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"Trilio.io"},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	// server private key
	serverPrivKey, err := rsa.GenerateKey(cryptorand.Reader, 4096)
	if err != nil {
		fmt.Println(err)
	}

	// sign the server cert
	serverCertBytes, err := x509.CreateCertificate(cryptorand.Reader, cert, ca, &serverPrivKey.PublicKey, caPrivKey)
	if err != nil {
		fmt.Println(err)
	}

	// PEM encode the  server cert and key
	serverCertPEM = new(bytes.Buffer)
	_ = pem.Encode(serverCertPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCertBytes,
	})

	serverPrivKeyPEM = new(bytes.Buffer)
	_ = pem.Encode(serverPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(serverPrivKey),
	})
	return caPEM, serverCertPEM, serverPrivKeyPEM
}

func GetHelmStorageBackendLabels() client.MatchingLabels {
	labels := client.MatchingLabels{"owner": "helm", "status": "deployed"}

	return labels
}

func MatchRegex(input, expr string) bool {
	if expr == "" {
		return true
	}
	log.Infof("matching regex[%s] with %s", expr, input)
	regexObj, err := regexp.Compile(expr)
	if err != nil {
		log.Errorf("failed to compile regex, invalid reqex %s", expr)
		return false
	}

	return regexObj.MatchString(input)
}

func CurrentTime() *metav1.Time {
	return &metav1.Time{Time: time.Now()}
}

func IsQueryStringMatches(str, query string) bool {
	return strings.Contains(strings.ToLower(str), strings.ToLower(query))
}

func CombinedReason(phase, status string) string {
	return phase + " " + status
}

func ParseDate(dateString string) (*metav1.Time, error) {
	t, err := time.Parse(ISODateFormat, dateString)
	if err != nil {
		return nil, err
	}
	return &metav1.Time{Time: t}, nil
}

// GetProfilingCollectorAddr gets the profiling collector address from the env var
func GetProfilingCollectorAddr() string {
	pc, present := os.LookupEnv(ProfilingCollector)
	if !present {
		return ""
	}
	return pc
}

// StartProfiling initiates the CPU profiling only if profiling collector address is available
func StartProfiling() {
	if profilingCollectorAddr := GetProfilingCollectorAddr(); profilingCollectorAddr != "" {
		if err := pprof.StartCPUProfile(&ProfilingBuffer); err != nil {
			log.Errorf("Failed while starting CPU profiling %s: ", err.Error())
		}
	}
}

// StopProfiling stops the CPU profiling and perform the heap profiling. It will send the collected
// profiling results to the profiling collector. This all will happen only if profiling collector
// is available in the env
func StopProfiling(action, ns string) {
	if profilingCollectorAddr := GetProfilingCollectorAddr(); profilingCollectorAddr != "" {
		pprof.StopCPUProfile()
		sendProfile(action+"-"+ns, "ns="+ns, profilingCollectorAddr, "cpu", &ProfilingBuffer)
		ProfilingBuffer.Reset()
		builtinruntime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(&ProfilingBuffer); err != nil {
			log.Errorf("Failed while writing memory profiling %s", err.Error())
			return
		}
		sendProfile(action+"-"+ns, "ns="+ns, profilingCollectorAddr, "heap", &ProfilingBuffer)
	}
}

// sendProfile will send the profiling data to the profiling collector
// Inputs params:
// service: Name by which the reports are getting pushed to collector
// labels: Labels to filter the data at the time of querying
// pc: Profiling collector address
// profileType: Type of the profiling i.e. cpu/heap
// buf: Buffer which holds the profiling data
// This function is ignoring the errors as the profiling feature is for internal use only
// nolint:interfacer Changing the bytes.Buffer to io.reader won't work as we are
// writing in the buffer before sending it here
func sendProfile(service, labels, pc, profileType string, buf *bytes.Buffer) {
	q := url.Values{}
	q.Set("service", service)
	q.Set("labels", labels)
	q.Set("type", profileType)

	sURL := pc + "/api/0/profiles?" + q.Encode()
	req, reqErr := http.NewRequest(http.MethodPost, sURL, buf)
	if reqErr != nil {
		log.Errorf("Error while creating request for %s %s %s", service, profileType, reqErr)
		return
	}

	res, resErr := http.DefaultClient.Do(req)
	if resErr != nil {
		log.Errorf("Error while sending %s %s profile to collector %s", service, profileType, resErr)
		return
	}
	log.Info(res)
}

func StartProfilingAgent(action, ns string) {
	if profilingCollectorAddr := GetProfilingCollectorAddr(); profilingCollectorAddr != "" {
		log.Info("Starting profefe profiling agent")
		_, err := agent.Start(profilingCollectorAddr, action+"-"+ns,
			agent.WithLabels("ns", ns),
			agent.WithLogger(func(s string, v ...interface{}) {
				log.Println(fmt.Sprintf(s, v...))
			}),
			agent.WithHeapProfile(),
			agent.WithTickInterval(ProfilingTickInterval*time.Second))
		if err != nil {
			log.Errorf("Failed to start the profefe agent: %v", err)
		}
	}
}
